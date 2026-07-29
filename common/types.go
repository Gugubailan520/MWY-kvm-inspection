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
	MsgTypeIfStats   MessageType = "ifstats"   // 网卡接口流量快照

	// Server -> Agent
	MsgTypeBlacklistSync MessageType = "blacklist_sync" // 黑名单同步
	MsgTypeBan           MessageType = "ban"            // 下发封禁指令
	MsgTypeUnban         MessageType = "unban"          // 下发解封指令
	MsgTypeIfStatsPush   MessageType = "ifstats_push"   // 向前端推送接口流量
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

// IfaceType 网络接口类型（参考 cockpit-traffic-monitor 的分类规则）
type IfaceType string

const (
	IfaceLoopback IfaceType = "loopback"
	IfaceEthernet IfaceType = "ethernet" // 物理 eth*/enp*/eno*/ens*/enx*
	IfaceBond     IfaceType = "bond"     // bond*
	IfaceVLAN     IfaceType = "vlan"     // eth0.10 形式
	IfaceBridge   IfaceType = "bridge"   // vmbr*/br-*/virbr*
	IfaceFirewall IfaceType = "firewall" // fwpr*/fwn*/fwln*/fwbr*
	IfaceTAP      IfaceType = "tap"      // tap*/tun*
	IfaceVeth     IfaceType = "veth"     // veth*
	IfaceWireless IfaceType = "wireless" // wlan*/wlp*/wls*/wlo*/wlx*
	IfaceVirtual  IfaceType = "virtual"  // docker*/wg*/ppp*
)

// ClassifyIface 按接口名判定类型（与 cockpit-traffic-monitor 的 ifaceType 对齐）
func ClassifyIface(name string) IfaceType {
	switch {
	case name == "lo":
		return IfaceLoopback
	case isPrefix(name, "bond"):
		return IfaceBond
	case isPrefix(name, "vmbr") || isPrefix(name, "br-") || isPrefix(name, "virbr"):
		return IfaceBridge
	case isPrefix(name, "fwpr") || isPrefix(name, "fwn") || isPrefix(name, "fwln") ||
		isPrefix(name, "fwbr") || isPrefix(name, "fwp") || isPrefix(name, "fwt"):
		return IfaceFirewall
	case isPrefix(name, "tap") || isPrefix(name, "tun"):
		return IfaceTAP
	case isPrefix(name, "veth"):
		return IfaceVeth
	case isPrefix(name, "wlan") || isPrefix(name, "wlp") || isPrefix(name, "wls") ||
		isPrefix(name, "wlo") || isPrefix(name, "wlx"):
		return IfaceWireless
	case isPrefix(name, "docker") || isPrefix(name, "wg") || isPrefix(name, "ppp"):
		return IfaceVirtual
	case isVLANSub(name):
		return IfaceVLAN
	case isPrefix(name, "eth") || isPrefix(name, "enp") || isPrefix(name, "eno") ||
		isPrefix(name, "ens") || isPrefix(name, "enx"):
		return IfaceEthernet
	default:
		return IfaceEthernet
	}
}

// isVLANSub 形如 eth0.10 / enp1s0.100
func isVLANSub(name string) bool {
	dot := -1
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 {
		return false
	}
	parent := name[:dot]
	// 排除本身就是特殊前缀的情况
	for _, p := range []string{"veth", "tap", "fw", "br-", "virbr"} {
		if isPrefix(parent, p) {
			return false
		}
	}
	// 点后必须全是数字
	for i := dot + 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}

func isPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

// IfaceStat 单个网络接口的一次流量快照（由 Agent 采集，Server 落库 + 推送）
type IfaceStat struct {
	Timestamp time.Time `json:"timestamp"    bson:"timestamp"`
	NodeID    string    `json:"node_id"      bson:"node_id"`
	Name      string    `json:"name"         bson:"name"`
	Type      IfaceType `json:"type"         bson:"type"`
	Up        bool      `json:"up"           bson:"up"`
	RxBytes   int64     `json:"rx_bytes"     bson:"rx_bytes"` // 累计接收字节
	TxBytes   int64     `json:"tx_bytes"     bson:"tx_bytes"` // 累计发送字节
	RxPackets int64     `json:"rx_packets"   bson:"rx_packets"`
	TxPackets int64     `json:"tx_packets"   bson:"tx_packets"`
	RxErrors  int64     `json:"rx_errors"    bson:"rx_errors"`
	TxErrors  int64     `json:"tx_errors"    bson:"tx_errors"`
	RxDropped int64     `json:"rx_dropped"   bson:"rx_dropped"`
	TxDropped int64     `json:"tx_dropped"   bson:"tx_dropped"`
	RxSpeed   float64   `json:"rx_speed"     bson:"rx_speed"` // 实时速率 bytes/s（由 Agent 计算）
	TxSpeed   float64   `json:"tx_speed"     bson:"tx_speed"`
	MTU       int       `json:"mtu,omitempty"   bson:"mtu,omitempty"`
	MAC       string    `json:"mac,omitempty"   bson:"mac,omitempty"`
	IPv4      string    `json:"ipv4,omitempty"  bson:"ipv4,omitempty"`
	IPv6      string    `json:"ipv6,omitempty"  bson:"ipv6,omitempty"`
	LinkSpeed int       `json:"link_speed,omitempty" bson:"link_speed,omitempty"` // Mbit/s
}

// IfStatsPayload 一批接口快照（Agent 一次上报）
type IfStatsPayload struct {
	NodeID    string      `json:"node_id"`
	Timestamp time.Time   `json:"timestamp"`
	Stats     []IfaceStat `json:"stats"`
}
