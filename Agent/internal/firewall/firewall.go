// Package firewall 封装对 iptables/ip6tables 的调用，用于在宿主机封禁 IP/域名。
package firewall

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
)

// Manager 管理封禁规则
type Manager struct {
	chain  string
	v4bin  string
	v6bin  string
	mu     sync.Mutex
	active map[string]bool // 已封禁 target 去重
}

// New 创建管理器
func New(chain, binaryPath string) *Manager {
	if chain == "" {
		chain = "FORWARD"
	}
	if binaryPath == "" {
		binaryPath = "/sbin/iptables"
	}
	v6 := strings.Replace(binaryPath, "iptables", "ip6tables", 1)
	return &Manager{chain: chain, v4bin: binaryPath, v6bin: v6, active: make(map[string]bool)}
}

// Ban 封禁一个 IP（域名需先解析为 IP）。
func (m *Manager) Ban(target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active[target] {
		return nil
	}
	if err := m.applyBan(target, m.v4bin); err != nil {
		return err
	}
	if isIPv6(target) {
		if err := m.applyBan(target, m.v6bin); err != nil {
			return err
		}
	}
	m.active[target] = true
	return nil
}

func (m *Manager) applyBan(target, bin string) error {
	// 对于 IPv4 用 iptables，IPv6 用 ip6tables；非 IP 地址跳过。
	if isIPv6(target) && bin == m.v4bin {
		return nil
	}
	if !isIPv6(target) && bin == m.v6bin {
		return nil
	}
	if !isIP(target) {
		// 非直接 IP，由上层解析后调用
		return fmt.Errorf("target is not an ip: %s", target)
	}
	cmd := exec.Command(bin, "-I", m.chain, "-d", target, "-j", "DROP")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s -I: %w: %s", bin, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Unban 解封
func (m *Manager) Unban(target string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active[target] {
		return nil
	}
	if isIP(target) && !isIPv6(target) {
		_ = m.applyUnban(target, m.v4bin)
	}
	if isIPv6(target) {
		_ = m.applyUnban(target, m.v6bin)
	}
	delete(m.active, target)
	return nil
}

func (m *Manager) applyUnban(target, bin string) error {
	cmd := exec.Command(bin, "-D", m.chain, "-d", target, "-j", "DROP")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s -D: %w: %s", bin, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Active 返回当前生效中的 target 列表（副本）
func (m *Manager) Active() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.active))
	for k := range m.active {
		out = append(out, k)
	}
	return out
}

func isIP(s string) bool { return net.ParseIP(s) != nil }
func isIPv6(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() == nil
}
