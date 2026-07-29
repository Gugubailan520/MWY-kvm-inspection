// Package api 实现 RESTful API 与 WebSocket 端点。
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/kvm-inspection/Server/internal/auth"
	"github.com/kvm-inspection/Server/internal/ifstatstore"
	"github.com/kvm-inspection/Server/internal/logstore"
	"github.com/kvm-inspection/Server/internal/model"
	"github.com/kvm-inspection/Server/internal/service"
	"github.com/kvm-inspection/Server/internal/violation"
	"github.com/kvm-inspection/Server/internal/ws"
	"github.com/kvm-inspection/common"
)

// Handlers 聚合所有路由处理器
type Handlers struct {
	svc      *service.Service
	logs     *logstore.Store
	ifs      *ifstatstore.Store
	engine   *violation.Engine
	hub      *ws.Hub
	auth     *auth.Manager
	upgrader websocket.Upgrader
}

// New 构造
func New(svc *service.Service, logs *logstore.Store, ifs *ifstatstore.Store, engine *violation.Engine, hub *ws.Hub, am *auth.Manager) *Handlers {
	return &Handlers{
		svc: svc, logs: logs, ifs: ifs, engine: engine, hub: hub, auth: am,
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
	}
}

// Register 注册路由到 engine
func (h *Handlers) Register(r *gin.Engine, corsOrigins []string) {
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.POST("/login", h.login)

		// 以下接口需要登录
		authd := api.Group("/")
		authd.Use(h.authMiddleware())
		{
			authd.GET("/me", h.me)
			authd.GET("/nodes", h.listNodes)
			authd.POST("/nodes", h.createNode)
			authd.DELETE("/nodes/:id", h.deleteNode)

			authd.GET("/events", h.listEvents)
			authd.GET("/events/export", h.exportEvents)

			authd.GET("/rules", h.listRules)
			authd.POST("/rules", h.createRule)
			authd.PUT("/rules/:id", h.updateRule)
			authd.DELETE("/rules/:id", h.deleteRule)

			authd.GET("/blacklist", h.listBlacklist)
			authd.POST("/blacklist", h.addBlacklist)
			authd.DELETE("/blacklist/:id", h.deleteBlacklist)

			authd.GET("/ifstats", h.ifStatsLatest)
			authd.GET("/ifstats/history", h.ifStatsHistory)

			authd.GET("/dashboard", h.dashboard)
		}

		// 前端实时推送
		r.GET("/ws", h.frontendWS)
		// Agent 长连接
		r.GET("/agent", h.agentWS)
	}
}

// ---------- middleware ----------

func (h *Handlers) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if len(raw) > 7 && raw[:7] == "Bearer " {
			raw = raw[7:]
		}
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := h.auth.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set("uid", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// ---------- auth ----------

func (h *Handlers) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	tok, err := h.auth.Issue(u.ID, u.Username, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok, "role": u.Role, "username": u.Username})
}

func (h *Handlers) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"uid":      c.GetUint64("uid"),
		"username": c.GetString("username"),
		"role":     c.GetString("role"),
	})
}

// ---------- nodes ----------

func (h *Handlers) listNodes(c *gin.Context) {
	ns, err := h.svc.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": ns, "total": len(ns)})
}

func (h *Handlers) createNode(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		IP   string `json:"ip"`
		OS   string `json:"os"`
		Virt string `json:"virt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := h.svc.CreateNode(req.Name, req.IP, req.OS, req.Virt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handlers) deleteNode(c *gin.Context) {
	if err := h.svc.DeleteNode(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- events ----------

func (h *Handlers) listEvents(c *gin.Context) {
	q := logstore.Query{
		NodeID: c.Query("node_id"),
		VMID:   c.Query("vm_id"),
		DstIP:  c.Query("dst_ip"),
		Domain: c.Query("domain"),
		Limit:  atoiDefault(c.Query("limit"), 100),
		Skip:   atoiDefault(c.Query("skip"), 0),
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.From = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.To = t
		}
	}
	if v := c.Query("is_violation"); v != "" {
		b := v == "true" || v == "1"
		q.IsViolation = &b
	}
	items, total, err := h.logs.Query(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handlers) exportEvents(c *gin.Context) {
	q := logstore.Query{Limit: 10000, IsViolation: boolPtr(true)}
	items, _, err := h.logs.Query(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", `attachment; filename="violation_events.json"`)
	c.JSON(http.StatusOK, items)
}

// ---------- rules ----------

func (h *Handlers) listRules(c *gin.Context) {
	rs, err := h.svc.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rs})
}

func (h *Handlers) createRule(c *gin.Context) {
	var r model.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.CreateRule(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.reloadRules(c)
	c.JSON(http.StatusOK, r)
}

func (h *Handlers) updateRule(c *gin.Context) {
	id := uint(atoiDefault(c.Param("id"), 0))
	var r model.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r.ID = id
	if err := h.svc.UpdateRule(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.reloadRules(c)
	c.JSON(http.StatusOK, r)
}

func (h *Handlers) deleteRule(c *gin.Context) {
	id := uint(atoiDefault(c.Param("id"), 0))
	if err := h.svc.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.reloadRules(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handlers) reloadRules(c *gin.Context) {
	if rs, err := h.svc.ListRules(); err == nil {
		h.engine.Reload(rs)
	}
}

// ---------- blacklist ----------

func (h *Handlers) listBlacklist(c *gin.Context) {
	bs, err := h.svc.ListBlacklist()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": bs})
}

func (h *Handlers) addBlacklist(c *gin.Context) {
	var b model.Blacklist
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.AddBlacklist(&b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 推送给所有 Agent（IP 类型）
	if b.Kind == "ip" {
		h.hub.SendAllAgents(common.WSMessage{Type: common.MsgTypeBan, Payload: common.BanAction{
			Action: "ban", Target: b.Target, Kind: b.Kind,
		}})
	}
	c.JSON(http.StatusOK, b)
}

func (h *Handlers) deleteBlacklist(c *gin.Context) {
	id := uint(atoiDefault(c.Param("id"), 0))
	// 先查询用于解封下发
	var b model.Blacklist
	_ = h.svc.DB.First(&b, id).Error
	if err := h.svc.DeleteBlacklist(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if b.Kind == "ip" {
		h.hub.SendAllAgents(common.WSMessage{Type: common.MsgTypeUnban, Payload: common.BanAction{
			Action: "unban", Target: b.Target, Kind: b.Kind,
		}})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- interface traffic (ifstats) ----------

// ifStatsLatest 返回指定节点每个接口的最新一条快照（接口列表卡片用）
func (h *Handlers) ifStatsLatest(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		// 默认取任意在线节点
		if ids := h.hub.AgentIDs(); len(ids) > 0 {
			nodeID = ids[0]
		}
	}
	stats, err := h.ifs.Latest(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": stats, "node_id": nodeID})
}

// ifStatsHistory 返回某节点某接口的历史流量序列（绘趋势图用，时间正序）
func (h *Handlers) ifStatsHistory(c *gin.Context) {
	q := ifstatstore.IfStatQuery{
		NodeID: c.Query("node_id"),
		Name:   c.Query("name"),
		Limit:  atoiDefault(c.Query("limit"), 1000),
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.From = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.To = t
		}
	}
	stats, err := h.ifs.Query(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": stats})
}

// ---------- dashboard ----------

func (h *Handlers) dashboard(c *gin.Context) {
	ns, _ := h.svc.ListNodes()
	online := 0
	for _, n := range ns {
		if n.Status == "online" {
			online++
		}
	}
	_, totalVio, _ := h.logs.Query(c.Request.Context(), logstore.Query{IsViolation: boolPtr(true), Limit: 1})
	now := time.Now()
	_, dayVio, _ := h.logs.Query(c.Request.Context(), logstore.Query{IsViolation: boolPtr(true), From: now.Add(-24 * time.Hour), Limit: 1})

	c.JSON(http.StatusOK, gin.H{
		"node_total":  len(ns),
		"node_online": online,
		"vio_total":   totalVio,
		"vio_24h":     dayVio,
	})
}

// ---------- websocket ----------

func (h *Handlers) frontendWS(c *gin.Context) {
	// 简化：前端 WS 也要求登录（通过 query token）
	if _, err := h.auth.Parse(c.Query("token")); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	fc := h.hub.RegisterFrontend(conn)
	defer func() {
		h.hub.UnregisterFrontend(fc)
		conn.Close()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Handlers) agentWS(c *gin.Context) {
	nodeID := c.Query("node_id")
	apiKey := c.Query("api_key")
	if _, err := h.svc.AuthAgent(nodeID, apiKey); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid node credentials"})
		return
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	ac := h.hub.RegisterAgent(nodeID, conn)
	defer func() {
		h.hub.UnregisterAgent(nodeID, ac)
		svc := h.svc
		go svc.MarkOffline(nodeID)
		conn.Close()
	}()

	_ = h.svc.UpdateHeartbeat(nodeID, "")
	go h.hub.StartAgentWriteLoop(ac)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		h.handleAgentMessage(nodeID, data)
	}
}

func (h *Handlers) handleAgentMessage(nodeID string, data []byte) {
	var msg common.WSMessage
	if err := decodeJSON(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	case common.MsgTypeHeartbeat:
		_ = h.svc.UpdateHeartbeat(nodeID, "")
	case common.MsgTypeEvent:
		ev := &common.NetworkEvent{}
		if err := remap(msg.Payload, ev); err != nil {
			return
		}
		ev.NodeID = nodeID
		// 服务端规则二次判定
		if r := h.engine.Judge(ev); r.Hit {
			ev.IsViolation = true
			ev.ViolationType = r.Type
			ev.ViolationDetail = r.Detail
		}
		_ = h.logs.Insert(context.Background(), ev)
		// 推送给前端
		if ev.IsViolation {
			h.hub.BroadcastEvent(ev)
		}
	case common.MsgTypeIfStats:
		// 接口流量快照：落库 + 实时推送前端
		var payload common.IfStatsPayload
		if err := remap(msg.Payload, &payload); err != nil {
			return
		}
		for i := range payload.Stats {
			payload.Stats[i].NodeID = nodeID
		}
		if h.ifs != nil {
			_ = h.ifs.InsertMany(context.Background(), payload.Stats)
		}
		h.hub.BroadcastIfStats(&payload)
	}
}

// ---------- helpers ----------

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func boolPtr(b bool) *bool { return &b }

// 避免引入 encoding/json 反复 import
func decodeJSON(data []byte, v any) error { return jsonUnmarshal(data, v) }
func remap(src, dst any) error            { return jsonRemap(src, dst) }
