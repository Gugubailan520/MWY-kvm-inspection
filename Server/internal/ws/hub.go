// Package ws 实现 WebSocket Hub：
//  1. /agent  接受 Agent 长连接，校验 API Key，接收事件
//  2. /ws     接受前端长连接，广播实时事件给所有登录客户端
package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/kvm-inspection/common"
)

// Hub 统一管理所有连接
type Hub struct {
	mu        sync.RWMutex
	agents    map[string]*agentConn // nodeID -> conn
	frontends map[*frontendConn]struct{}
}

// agentConn Agent 连接
type agentConn struct {
	nodeID string
	conn   *websocket.Conn
	send   chan []byte // server -> agent 下发
}

// frontendConn 前端连接
type frontendConn struct {
	conn *websocket.Conn
}

// NewHub 创建
func NewHub() *Hub {
	return &Hub{
		agents:    make(map[string]*agentConn),
		frontends: make(map[*frontendConn]struct{}),
	}
}

// RegisterAgent 注册 Agent 连接
func (h *Hub) RegisterAgent(nodeID string, conn *websocket.Conn) *agentConn {
	ac := &agentConn{nodeID: nodeID, conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	if old := h.agents[nodeID]; old != nil {
		_ = old.conn.Close()
	}
	h.agents[nodeID] = ac
	h.mu.Unlock()
	return ac
}

// UnregisterAgent 注销
func (h *Hub) UnregisterAgent(nodeID string, ac *agentConn) {
	h.mu.Lock()
	if cur := h.agents[nodeID]; cur == ac {
		delete(h.agents, nodeID)
	}
	h.mu.Unlock()
	close(ac.send)
}

// RegisterFrontend 注册前端连接
func (h *Hub) RegisterFrontend(conn *websocket.Conn) *frontendConn {
	fc := &frontendConn{conn: conn}
	h.mu.Lock()
	h.frontends[fc] = struct{}{}
	h.mu.Unlock()
	return fc
}

// UnregisterFrontend 注销
func (h *Hub) UnregisterFrontend(fc *frontendConn) {
	h.mu.Lock()
	delete(h.frontends, fc)
	h.mu.Unlock()
}

// AgentIDs 当前在线 Agent 列表
func (h *Hub) AgentIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.agents))
	for id := range h.agents {
		out = append(out, id)
	}
	return out
}

// BroadcastEvent 向所有前端连接广播一条事件
func (h *Hub) BroadcastEvent(ev *common.NetworkEvent) {
	data, err := json.Marshal(common.WSMessage{Type: common.MsgTypeEvent, Payload: ev})
	if err != nil {
		return
	}
	h.mu.RLock()
	targets := make([]*frontendConn, 0, len(h.frontends))
	for fc := range h.frontends {
		targets = append(targets, fc)
	}
	h.mu.RUnlock()

	for _, fc := range targets {
		_ = fc.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := fc.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.UnregisterFrontend(fc)
		}
	}
}

// SendToAgent 向指定 Agent 下发消息（黑名单/封禁指令）
func (h *Hub) SendToAgent(nodeID string, msg common.WSMessage) bool {
	h.mu.RLock()
	ac := h.agents[nodeID]
	h.mu.RUnlock()
	if ac == nil {
		return false
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	select {
	case ac.send <- data:
		return true
	default:
		log.Printf("[hub] agent %s send queue full", nodeID)
		return false
	}
}

// SendAllAgents 向所有 Agent 下发
func (h *Hub) SendAllAgents(msg common.WSMessage) {
	h.mu.RLock()
	ids := make([]string, 0, len(h.agents))
	for id := range h.agents {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	for _, id := range ids {
		h.SendToAgent(id, msg)
	}
}

// StartAgentWriteLoop 把下发队列里的消息写入 Agent 连接。
func (h *Hub) StartAgentWriteLoop(ac *agentConn) {
	for data := range ac.send {
		_ = ac.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := ac.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}
