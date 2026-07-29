// kvm-agent 入口
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kvm-inspection/Agent/internal/capture"
	"github.com/kvm-inspection/Agent/internal/config"
	"github.com/kvm-inspection/Agent/internal/firewall"
	"github.com/kvm-inspection/Agent/internal/ifstat"
	"github.com/kvm-inspection/Agent/internal/reporter"
	"github.com/kvm-inspection/Agent/internal/storage"
	"github.com/kvm-inspection/common"
)

const version = "0.1.0"

func main() {
	cfgPath := flag.String("c", "config/agent.local.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.NodeID == "" {
		log.Fatal("node_id is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1) 防火墙管理器
	var fw *firewall.Manager
	if cfg.Firewall.Enable {
		fw = firewall.New(cfg.Firewall.Chain, cfg.Firewall.BinaryPath)
	}

	// 2) Reporter（含黑名单同步）
	rep := reporter.New(reporter.Config{
		NodeID:       cfg.NodeID,
		APIKey:       cfg.APIKey,
		Hostname:     cfg.Hostname,
		ServerURL:    cfg.Server.URL,
		ReconnectSec: cfg.Server.ReconnectSec,
		Version:      version,
		Firewall:     fw,
	}, cfg.Report.BufferCap)

	go rep.Run(ctx)

	// 3) 本地缓冲存储（断网时）
	var st *storage.SQLite
	if cfg.Storage.Type == "sqlite" {
		st, err = storage.NewSQLite(cfg.Storage.SQLite.Path)
		if err != nil {
			log.Printf("open sqlite buffer: %v", err)
		}
	}

	// 4) 事件汇聚通道
	evCh := make(chan *common.NetworkEvent, cfg.Report.BufferCap)

	// 5) 抓包循环：每个网卡一个 Sniffer（Parser 内部已自带 DPI 引擎）
	parser := capture.NewParser(cfg.NodeID)
	var sniffers []*capture.Sniffer
	ifaces := cfg.Capture.Interfaces
	if len(ifaces) == 0 {
		log.Printf("[warn] no capture interfaces configured, agent will run in dry mode")
	}
	for _, iface := range ifaces {
		s := capture.NewSniffer(iface, cfg.Capture.SnapLen, cfg.Capture.Promisc, cfg.Capture.BPFFilter, parser)
		sniffers = append(sniffers, s)
		go func(iface string) {
			if err := s.Run(evCh); err != nil {
				log.Printf("sniffer %s: %v", iface, err)
			}
		}(iface)
	}

	// 6) 事件加工 + 上报循环：把事件投递给 reporter；断网时落本地 SQLite
	go pipeline(ctx, evCh, rep, st)

	// 7) 接口流量监控（参考 cockpit-traffic-monitor）：定时采集 /proc/net/dev 上报
	if cfg.IfStat.Enable {
		coll := ifstat.New(cfg.NodeID)
		ifStatsCh := make(chan *common.IfStatsPayload, 16)
		go coll.Run(time.Duration(cfg.IfStat.IntervalSec)*time.Second, ifStatsCh, ctx.Done())
		go forwardIfStats(ifStatsCh, rep)
		log.Printf("[agent] ifstat enabled, interval=%ds", cfg.IfStat.IntervalSec)
	}

	log.Printf("[agent] node_id=%s started, version=%s, interfaces=%v", cfg.NodeID, version, ifaces)

	<-ctx.Done()
	log.Printf("[agent] shutting down...")
	for _, s := range sniffers {
		s.Stop()
	}
	rep.Stop()
	if st != nil {
		_ = st.Close()
	}
}

// pipeline 把事件投递给 reporter；队列满则落本地 SQLite 缓冲，待重连补传。
func pipeline(ctx context.Context, evCh <-chan *common.NetworkEvent, rep *reporter.Reporter, st *storage.SQLite) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-evCh:
			if rep.Enqueue(ev) {
				if st != nil {
					if err := st.Push(ctx, ev); err != nil {
						log.Printf("buffer push: %v", err)
					}
				}
			}
		}
	}
}

// forwardIfStats 把采集到的接口流量快照通过 reporter 上报给服务端。
func forwardIfStats(ch <-chan *common.IfStatsPayload, rep *reporter.Reporter) {
	for p := range ch {
		msg := &common.WSMessage{Type: common.MsgTypeIfStats, Payload: p}
		rep.SendMessage(msg)
	}
}
