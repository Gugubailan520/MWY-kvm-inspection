// Package capture 负责从宿主机网卡实时抓包，并拆解成 common.NetworkEvent。
package capture

import (
	"fmt"
	"sync/atomic"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/kvm-inspection/Agent/internal/dpi"
	"github.com/kvm-inspection/common"
)

// Parser 把原始数据包拆解为 NetworkEvent。
type Parser struct {
	NodeID string
	dpi    *dpi.Engine
}

// NewParser 构造解析器
func NewParser(nodeID string) *Parser { return &Parser{NodeID: nodeID, dpi: dpi.NewEngine()} }

// Handle 处理单包，返回解析得到的事件（可能为 nil）。
func (p *Parser) Handle(packet gopacket.Packet) *common.NetworkEvent {
	netLayer := packet.NetworkLayer()
	if netLayer == nil {
		return nil
	}
	ev := &common.NetworkEvent{
		Timestamp: packet.Metadata().Timestamp,
		NodeID:    p.NodeID,
	}

	switch l := netLayer.(type) {
	case *layers.IPv4:
		ev.SrcIP = l.SrcIP.String()
		ev.DstIP = l.DstIP.String()
	case *layers.IPv6:
		ev.SrcIP = l.SrcIP.String()
		ev.DstIP = l.DstIP.String()
	default:
		return nil
	}

	ev.BytesSent, ev.BytesReceived = int64(len(packet.Data())), 0

	tpLayer := packet.TransportLayer()
	if tpLayer != nil {
		switch l := tpLayer.(type) {
		case *layers.TCP:
			ev.Protocol = "TCP"
			ev.SrcPort = int(l.SrcPort)
			ev.DstPort = int(l.DstPort)
			app := packet.ApplicationLayer()
			payload := []byte(nil)
			if app != nil {
				payload = app.Payload()
				p.enrichFromAppLayer(ev, payload)
			}
			// 对握手包做 DPI 识别（仅 SYN 之后的携带数据包才有可能）
			if payload != nil && len(payload) > 0 {
				if r := p.dpi.Analyze(payload, l); r.Detected {
					ev.DetectedProtocol = r.Protocol
					// 识别到代理协议直接标违规
					ev.IsViolation = true
					ev.ViolationType = "proxy_protocol"
					ev.ViolationDetail = r.Detail
				}
			}
		case *layers.UDP:
			ev.Protocol = "UDP"
			ev.SrcPort = int(l.SrcPort)
			ev.DstPort = int(l.DstPort)
			app := packet.ApplicationLayer()
			if app != nil {
				p.enrichFromDNS(ev, app.Payload())
			}
		default:
			ev.Protocol = tpLayer.LayerType().String()
		}
	}

	// 默认认为出站（本机网卡发出）。精确判定需要本地 IP 集合，这里简化。
	ev.Direction = common.DirectionOutbound
	return ev
}

// enrichFromAppLayer 解析 HTTP 等应用层，提取标题。
func (p *Parser) enrichFromAppLayer(ev *common.NetworkEvent, payload []byte) {
	if title, ok := extractHTMLTitle(payload); ok {
		ev.Title = title
	}
}

// enrichFromDNS 解析 DNS 查询/响应，提取域名。
func (p *Parser) enrichFromDNS(ev *common.NetworkEvent, payload []byte) {
	var dns layers.DNS
	if err := dns.DecodeFromBytes(payload, gopacket.NilDecodeFeedback); err == nil {
		if dns.Questions != nil && len(dns.Questions) > 0 {
			ev.Domain = string(dns.Questions[0].Name)
		}
	}
}

// Sniffer 封装单网卡抓包循环。
type Sniffer struct {
	iface   string
	snapLen int32
	promisc bool
	filter  string
	parser  *Parser
	stop    atomic.Bool
}

// NewSniffer 创建一个网卡抓包器
func NewSniffer(iface string, snapLen int32, promisc bool, bpf string, parser *Parser) *Sniffer {
	return &Sniffer{iface: iface, snapLen: snapLen, promisc: promisc, filter: bpf, parser: parser}
}

// Stop 停止抓包循环
func (s *Sniffer) Stop() { s.stop.Store(true) }

// Run 阻塞抓包，把事件写入 out 通道。
func (s *Sniffer) Run(out chan<- *common.NetworkEvent) error {
	handle, err := pcap.OpenLive(s.iface, s.snapLen, s.promisc, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("open %s: %w", s.iface, err)
	}
	defer handle.Close()

	if s.filter != "" {
		if err := handle.SetBPFFilter(s.filter); err != nil {
			return fmt.Errorf("apply bpf filter: %w", err)
		}
	}

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	for !s.stop.Load() {
		pkt, ok := <-src.Packets()
		if !ok {
			return nil
		}
		if ev := s.parser.Handle(pkt); ev != nil {
			select {
			case out <- ev:
			default:
				// 通道满则丢弃，避免阻塞抓包线程，可加监控指标。
			}
		}
	}
	return nil
}
