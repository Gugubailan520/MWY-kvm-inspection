// Package violation 是违规判定引擎。
// 在服务端再次校验事件是否命中规则库，与 Agent 侧 DPI 形成双重判定。
package violation

import (
	"net"
	"strings"
	"sync"

	"github.com/kvm-inspection/Server/internal/model"
	"github.com/kvm-inspection/common"
)

// Engine 规则引擎
type Engine struct {
	mu    sync.RWMutex
	rules []model.Rule
}

// New 创建引擎
func New() *Engine { return &Engine{} }

// Reload 更新规则集
func (e *Engine) Reload(rules []model.Rule) {
	enabled := make([]model.Rule, 0, len(rules))
	for _, r := range rules {
		if r.Enabled {
			enabled = append(enabled, r)
		}
	}
	e.mu.Lock()
	e.rules = enabled
	e.mu.Unlock()
}

// Result 判定结果
type Result struct {
	Hit    bool
	Type   string
	Detail string
}

// Judge 对单事件做规则匹配。事件本身已被 Agent 标记的违规会先于规则校验。
func (e *Engine) Judge(ev *common.NetworkEvent) Result {
	if ev == nil {
		return Result{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range e.rules {
		switch r.Type {
		case "blacklist_ip":
			if matchBlacklistIP(ev, r.Pattern) {
				return Result{Hit: true, Type: "blacklist_ip", Detail: "访问黑名单IP: " + ev.DstIP}
			}
		case "blacklist_domain":
			if matchDomain(ev.Domain, r.Pattern) {
				return Result{Hit: true, Type: "blacklist_domain", Detail: "访问黑名单域名: " + ev.Domain}
			}
		case "port":
			if matchPort(ev.DstPort, r.Pattern) {
				return Result{Hit: true, Type: "port", Detail: "连接违规端口: " + itoa(ev.DstPort)}
			}
		case "keyword":
			if containsKeyword(ev.Title, r.Pattern) {
				return Result{Hit: true, Type: "keyword", Detail: "网页标题命中关键词: " + ev.Title}
			}
		case "protocol":
			if ev.DetectedProtocol != "" && strings.EqualFold(ev.DetectedProtocol, r.Pattern) {
				return Result{Hit: true, Type: "proxy_protocol", Detail: "识别到代理协议: " + ev.DetectedProtocol}
			}
		}
	}
	// Agent 已经标记的违规（如 DPI 命中）直接采纳
	if ev.IsViolation {
		return Result{Hit: true, Type: ev.ViolationType, Detail: ev.ViolationDetail}
	}
	return Result{}
}

func matchBlacklistIP(ev *common.NetworkEvent, pattern string) bool {
	for _, p := range strings.Split(pattern, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "/") {
			_, ipnet, err := net.ParseCIDR(p)
			if err == nil {
				ip := net.ParseIP(ev.DstIP)
				if ip != nil && ipnet.Contains(ip) {
					return true
				}
			}
			continue
		}
		if p == ev.DstIP {
			return true
		}
	}
	return false
}

// matchDomain 支持 *.example.com 通配符
func matchDomain(domain, pattern string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, p := range strings.Split(pattern, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*.") {
			suffix := p[1:] // .example.com
			if strings.HasSuffix(domain, suffix) {
				return true
			}
			continue
		}
		if domain == p {
			return true
		}
	}
	return false
}

func matchPort(port int, pattern string) bool {
	for _, p := range strings.Split(pattern, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n := atoi(p); n > 0 && n == port {
			return true
		}
	}
	return false
}

func containsKeyword(title, keywords string) bool {
	if title == "" {
		return false
	}
	t := strings.ToLower(title)
	for _, k := range strings.Split(keywords, ",") {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" && strings.Contains(t, k) {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	// 简易实现，避免 import strconv 的同时也保持可读
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
