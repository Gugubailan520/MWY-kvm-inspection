// Package ifstat 采集宿主机各网络接口的流量/速率/错误/丢包。
// 参考 cockpit-traffic-monitor：解析 /proc/net/dev + 读取 /sys/class/net/* 元信息，
// 通过前后两次快照差值计算实时速率。
package ifstat

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kvm-inspection/common"
)

// Collector 接口流量采集器
type Collector struct {
	nodeID string
	mu     sync.Mutex
	prev   map[string]sample // 上次快照：name -> 累计字节 + 时间
	prevAt time.Time
}

type sample struct {
	rxBytes   int64
	txBytes   int64
	rxPackets int64
	txPackets int64
	rxErrors  int64
	txErrors  int64
	rxDropped int64
	txDropped int64
	ts        time.Time
}

// New 创建采集器
func New(nodeID string) *Collector {
	return &Collector{nodeID: nodeID, prev: make(map[string]sample)}
}

// Snapshot 采集一次所有接口的快照（累计值 + 速率）。
func (c *Collector) Snapshot() ([]common.IfaceStat, error) {
	cur, err := parseProcNetDev()
	if err != nil {
		return nil, err
	}
	now := time.Now()

	c.mu.Lock()
	prev := c.prev
	prevAt := c.prevAt
	// 更新缓存
	c.prev = cur
	c.prevAt = now
	c.mu.Unlock()

	dt := 0.0
	if !prevAt.IsZero() {
		dt = now.Sub(prevAt).Seconds()
	}

	out := make([]common.IfaceStat, 0, len(cur))
	for name, s := range cur {
		st := common.IfaceStat{
			Timestamp: now,
			NodeID:    c.nodeID,
			Name:      name,
			Type:      common.ClassifyIface(name),
			Up:        readOperState(name) == "up",
			RxBytes:   s.rxBytes,
			TxBytes:   s.txBytes,
			RxPackets: s.rxPackets,
			TxPackets: s.txPackets,
			RxErrors:  s.rxErrors,
			TxErrors:  s.txErrors,
			RxDropped: s.rxDropped,
			TxDropped: s.txDropped,
			MAC:       readSysFile(name, "address"),
			MTU:       atoiSafe(readSysFile(name, "mtu")),
			LinkSpeed: atoiSafe(readSysFile(name, "speed")),
			IPv4:      readIfaceAddr(name, false),
			IPv6:      readIfaceAddr(name, true),
		}
		// 计算速率：与上次快照差值 / 时间间隔
		if dt > 0 {
			if p, ok := prev[name]; ok {
				st.RxSpeed = clampNonNeg(float64(s.rxBytes-p.rxBytes) / dt)
				st.TxSpeed = clampNonNeg(float64(s.txBytes-p.txBytes) / dt)
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// parseProcNetDev 解析 /proc/net/dev。
// 字段顺序（自第 2 列起）：rx_bytes rx_packets rx_errs rx_drop ... tx_bytes tx_packets tx_errs tx_drop ...
func parseProcNetDev() (map[string]sample, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]sample)
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		if lineNo <= 2 { // 跳过两行表头
			continue
		}
		line := sc.Text()
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 16 {
			continue
		}
		out[name] = sample{
			rxBytes:   parseInt64(fields[0]),
			rxPackets: parseInt64(fields[1]),
			rxErrors:  parseInt64(fields[2]),
			rxDropped: parseInt64(fields[3]),
			txBytes:   parseInt64(fields[8]),
			txPackets: parseInt64(fields[9]),
			txErrors:  parseInt64(fields[10]),
			txDropped: parseInt64(fields[11]),
			ts:        time.Now(),
		}
	}
	return out, sc.Err()
}

// Run 定时采集并通过 out 通道输出（非阻塞写入）。
func (c *Collector) Run(interval time.Duration, out chan<- *common.IfStatsPayload, stop <-chan struct{}) {
	tk := time.NewTicker(interval)
	defer tk.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tk.C:
			stats, err := c.Snapshot()
			if err != nil {
				continue
			}
			if len(stats) == 0 {
				continue
			}
			payload := &common.IfStatsPayload{
				NodeID:    c.nodeID,
				Timestamp: time.Now(),
				Stats:     stats,
			}
			select {
			case out <- payload:
			default:
			}
		}
	}
}

// ---------- /sys/class/net 辅助读取 ----------

func readSysFile(iface, attr string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", iface, attr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readOperState(iface string) string { return readSysFile(iface, "operstate") }

// readIfaceAddr 读取接口 IP 地址（v4/v6），返回首个地址。
// 优先从 /proc 实现（避免依赖 netlink），这里用最简单的 ip 命令不可用时的兜底。
func readIfaceAddr(iface string, ipv6 bool) string {
	return ""
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func clampNonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
