// Package config 定义 Agent 的配置加载逻辑。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config Agent 配置
type Config struct {
	NodeID   string   `yaml:"node_id"`
	APIKey   string   `yaml:"api_key"`
	Hostname string   `yaml:"hostname"`
	Server   Server   `yaml:"server"`
	Capture  Capture  `yaml:"capture"`
	Storage  Storage  `yaml:"storage"`
	Report   Report   `yaml:"report"`
	Firewall Firewall `yaml:"firewall"`
	IfStat   IfStat   `yaml:"ifstat"`
}

// IfStat 接口流量监控配置（参考 cockpit-traffic-monitor）
type IfStat struct {
	Enable      bool `yaml:"enable"`       // 默认 true
	IntervalSec int  `yaml:"interval_sec"` // 采集间隔，默认 2
}

type Server struct {
	URL           string `yaml:"url"`           // ws://host:port/agent
	TLSCACertPath string `yaml:"tls_ca_cert"`   // 可选
	ReconnectSec  int    `yaml:"reconnect_sec"` // 重连间隔，默认 5
}

type Capture struct {
	Interfaces []string `yaml:"interfaces"`  // 监听网卡，留空自动检测
	BPFFilter  string   `yaml:"bpf_filter"`  // BPF 过滤表达式
	SnapLen    int32    `yaml:"snap_len"`    // 抓包长度，默认 65535
	Promisc    bool     `yaml:"promiscuous"` // 混杂模式
}

type Storage struct {
	Type   string `yaml:"type"` // sqlite | json
	SQLite struct {
		Path      string `yaml:"path"`
		MaxSizeMB int    `yaml:"max_size_mb"`
	} `yaml:"sqlite"`
}

type Report struct {
	BatchSize int `yaml:"batch_size"` // 批量上报大小，默认 100
	FlushSec  int `yaml:"flush_sec"`  // 刷新间隔，默认 3
	BufferCap int `yaml:"buffer_cap"` // 事件缓冲通道容量，默认 10000
}

type Firewall struct {
	Enable     bool   `yaml:"enable"`
	Chain      string `yaml:"chain"` // 默认 FORWARD
	BinaryPath string `yaml:"binary_path"`
}

// Load 从 yaml 文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Server.ReconnectSec <= 0 {
		c.Server.ReconnectSec = 5
	}
	if c.Capture.SnapLen <= 0 {
		c.Capture.SnapLen = 65535
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "sqlite"
	}
	if c.Storage.SQLite.Path == "" {
		c.Storage.SQLite.Path = "./agent_buffer.db"
	}
	if c.Storage.SQLite.MaxSizeMB <= 0 {
		c.Storage.SQLite.MaxSizeMB = 500
	}
	if c.Report.BatchSize <= 0 {
		c.Report.BatchSize = 100
	}
	if c.Report.FlushSec <= 0 {
		c.Report.FlushSec = 3
	}
	if c.Report.BufferCap <= 0 {
		c.Report.BufferCap = 10000
	}
	if c.Firewall.Chain == "" {
		c.Firewall.Chain = "FORWARD"
	}
	if c.Firewall.BinaryPath == "" {
		c.Firewall.BinaryPath = "/sbin/iptables"
	}
	// ifstat 默认开启
	if c.IfStat.IntervalSec <= 0 {
		c.IfStat.IntervalSec = 2
	}
}
