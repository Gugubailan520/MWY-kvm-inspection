// Package common 提供 Agent 与 Server 共享的类型定义。
package common

import "time"

// Direction 流量方向
type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

// NetworkEvent 是 Agent 上报、Server 落库 MongoDB 的网络事件结构。
type NetworkEvent struct {
	Timestamp        time.Time `json:"timestamp"          bson:"timestamp"`
	NodeID           string    `json:"node_id"            bson:"node_id"`
	VMID             string    `json:"vm_id"              bson:"vm_id"`
	SrcIP            string    `json:"src_ip"             bson:"src_ip"`
	SrcPort          int       `json:"src_port"           bson:"src_port"`
	DstIP            string    `json:"dst_ip"             bson:"dst_ip"`
	DstPort          int       `json:"dst_port"           bson:"dst_port"`
	Protocol         string    `json:"protocol"           bson:"protocol"`
	Direction        Direction `json:"direction"          bson:"direction"`
	Domain           string    `json:"domain,omitempty"   bson:"domain,omitempty"`
	Title            string    `json:"title,omitempty"    bson:"title,omitempty"`
	DetectedProtocol string    `json:"detected_protocol,omitempty" bson:"detected_protocol,omitempty"`
	BytesSent        int64     `json:"bytes_sent"         bson:"bytes_sent"`
	BytesReceived    int64     `json:"bytes_received"     bson:"bytes_received"`
	IsViolation      bool      `json:"is_violation"       bson:"is_violation"`
	ViolationType    string    `json:"violation_type,omitempty"     bson:"violation_type,omitempty"`
	ViolationDetail  string    `json:"violation_detail,omitempty"   bson:"violation_detail,omitempty"`
}

// MessageType Agent <-> Server WebSocket 消息类型
type MessageType string

const (
	// Agent -> Server
	MsgTypeEvent     MessageType = "event"     // 网络事件
	MsgTypeHeartbeat MessageType = "heartbeat" // 心跳

	// Server -> Agent
	MsgTypeBlacklistSync MessageType = "blacklist_sync" // 黑名单同步
	MsgTypeBan           MessageType = "ban"            // 下发封禁指令
	MsgTypeUnban         MessageType = "unban"          // 下发解封指令
)

// WSMessage WebSocket 传输信封
type WSMessage struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
}

// HeartbeatPayload 心跳消息
type HeartbeatPayload struct {
	NodeID     string    `json:"node_id"`
	Version    string    `json:"version"`
	Hostname   string    `json:"hostname"`
	CPUUsage   float64   `json:"cpu_usage"`
	MemUsageMB float64   `json:"mem_usage_mb"`
	ReportedAt time.Time `json:"reported_at"`
}

// BlacklistItem 黑名单条目
type BlacklistItem struct {
	ID        uint      `json:"id"`
	Target    string    `json:"target"` // IP 或域名
	Kind      string    `json:"kind"`   // ip / domain
	Action    string    `json:"action"` // drop / reject
	Status    string    `json:"status"` // active / inactive
	CreatedAt time.Time `json:"created_at"`
}

// BanAction 封禁/解封指令
type BanAction struct {
	Action string `json:"action"` // ban / unban
	Target string `json:"target"` // IP 或域名
	Kind   string `json:"kind"`   // ip / domain
}
