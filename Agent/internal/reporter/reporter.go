// Package reporter 负责与 Server 维持 WebSocket 长连接，
// 上报事件 / 心跳，并接收黑名单与封禁指令。
package reporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/kvm-inspection/Agent/internal/firewall"
	"github.com/kvm-inspection/common"
)

// Config reporter 配置
type Config struct {
	NodeID       string
	APIKey       string
	Hostname     string
	ServerURL    string
	ReconnectSec int
	Version      string
	Firewall     *firewall.Manager
	OnEvent      func(ctx context.Context, ev *common.NetworkEvent) // 事件入站（由本连接转发给 server）由外部驱动
}

// Reporter WebSocket 客户端
type Reporter struct {
	cfg   Config
	conn  *websocket.Conn
	mu    sync.Mutex
	sendQ chan *common.NetworkEvent
	stop  chan struct{}
	wg    sync.WaitGroup
}

// New 构造
func New(cfg Config, bufferCap int) *Reporter {
	return &Reporter{
		cfg:   cfg,
		sendQ: make(chan *common.NetworkEvent, bufferCap),
		stop:  make(chan struct{}),
	}
}

// Enqueue 把一个事件加入发送队列（非阻塞）
func (r *Reporter) Enqueue(ev *common.NetworkEvent) (dropped bool) {
	select {
	case r.sendQ <- ev:
		return false
	default:
		return true
	}
}

// Run 阻塞运行，自动重连
func (r *Reporter) Run(ctx context.Context) {
	r.wg.Add(1)
	go r.heartbeatLoop(ctx)

	for {
		// 先做一次非阻塞退出检查
		select {
		case <-ctx.Done():
			r.wg.Wait()
			return
		case <-r.stop:
			r.wg.Wait()
			return
		default:
		}

		if err := r.runOnce(ctx); err != nil {
			log.Printf("[reporter] connection error: %v", err)
		}
		wait(ctx, time.Duration(r.cfg.ReconnectSec)*time.Second)
	}
}

// runOnce 建立单次连接并维持收发循环。
func (r *Reporter) runOnce(ctx context.Context) error {
	u, err := url.Parse(r.cfg.ServerURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("node_id", r.cfg.NodeID)
	q.Set("api_key", r.cfg.APIKey)
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Set("User-Agent", "kvm-agent/"+r.cfg.Version)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()
	defer func() {
		conn.Close()
		r.mu.Lock()
		r.conn = nil
		r.mu.Unlock()
	}()
	log.Printf("[reporter] connected to %s", u.Host)

	// 读循环（服务端下发）
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.readLoop(ctx, conn)
	}()

	// 写循环（事件上报）
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return errors.New("read loop closed")
		case ev := <-r.sendQ:
			msg := common.WSMessage{Type: common.MsgTypeEvent, Payload: ev}
			if err := writeJSON(conn, msg); err != nil {
				// 写失败：把事件回退到队列，下次重连再发
				select {
				case r.sendQ <- ev:
				default:
				}
				return err
			}
		}
	}
}

func (r *Reporter) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[reporter] read: %v", err)
			return
		}
		r.handleServerMessage(data)
	}
}

func (r *Reporter) handleServerMessage(data []byte) {
	var msg common.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[reporter] bad message: %v", err)
		return
	}
	switch msg.Type {
	case common.MsgTypeBlacklistSync:
		if r.cfg.Firewall == nil {
			return
		}
		raw, _ := json.Marshal(msg.Payload)
		var items []common.BlacklistItem
		if err := json.Unmarshal(raw, &items); err == nil {
			r.applyBlacklist(items)
		}
	case common.MsgTypeBan:
		raw, _ := json.Marshal(msg.Payload)
		var act common.BanAction
		if err := json.Unmarshal(raw, &act); err == nil && r.cfg.Firewall != nil {
			if act.Action == "ban" {
				if err := r.cfg.Firewall.Ban(act.Target); err != nil {
					log.Printf("[reporter] ban %s: %v", act.Target, err)
				}
			} else {
				if err := r.cfg.Firewall.Unban(act.Target); err != nil {
					log.Printf("[reporter] unban %s: %v", act.Target, err)
				}
			}
		}
	}
}

func (r *Reporter) applyBlacklist(items []common.BlacklistItem) {
	want := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Status != "active" || it.Kind != "ip" {
			continue
		}
		want[it.Target] = true
	}
	for t := range want {
		_ = r.cfg.Firewall.Ban(t)
	}
	for _, t := range r.cfg.Firewall.Active() {
		if !want[t] {
			_ = r.cfg.Firewall.Unban(t)
		}
	}
}

func (r *Reporter) heartbeatLoop(ctx context.Context) {
	defer r.wg.Done()
	tk := time.NewTicker(30 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			r.mu.Lock()
			conn := r.conn
			r.mu.Unlock()
			if conn == nil {
				continue
			}
			hb := common.HeartbeatPayload{
				NodeID:     r.cfg.NodeID,
				Version:    r.cfg.Version,
				Hostname:   r.cfg.Hostname,
				ReportedAt: time.Now(),
			}
			_ = writeJSON(conn, common.WSMessage{Type: common.MsgTypeHeartbeat, Payload: hb})
		}
	}
}

// Stop 停止
func (r *Reporter) Stop() {
	close(r.stop)
}

func writeJSON(conn *websocket.Conn, v any) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

func wait(ctx context.Context, d time.Duration) {
	if d <= 0 {
		d = 5 * time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
