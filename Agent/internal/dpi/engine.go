// Package dpi 提供代理/隧道协议的深度包检测识别。
// 参考 OpenGFW（github.com/apernet/OpenGFW）的 analyzer 思路，实现轻量指纹匹配。
package dpi

import (
	"github.com/google/gopacket/layers"
)

// Analyzer 协议识别器接口。每个实现消费若干字节流，给出识别置信度。
type Analyzer interface {
	Name() string
	// Feed 投入一个 TCP 流的首包载荷，返回是否命中。
	Feed(payload []byte, tcp *layers.TCP) (matched bool, detail string)
}

// Engine DPI 引擎：管理多个 Analyzer，对单包做尽力识别。
type Engine struct {
	analyzers []Analyzer
}

// NewEngine 创建引擎并注册内置分析器。
func NewEngine() *Engine {
	return &Engine{analyzers: []Analyzer{
		&socks5Analyzer{}, &socks4Analyzer{},
		&httpProxyAnalyzer{}, &trojanAnalyzer{},
		&shadowsocksAnalyzer{}, &sshAnalyzer{},
	}}
}

// Result 识别结果
type Result struct {
	Detected bool
	Protocol string
	Detail   string
}

// Analyze 对单包 payload 做识别。命中后立即返回。
func (e *Engine) Analyze(payload []byte, tcp *layers.TCP) Result {
	if len(payload) == 0 {
		return Result{}
	}
	for _, a := range e.analyzers {
		if ok, detail := a.Feed(payload, tcp); ok {
			return Result{Detected: true, Protocol: a.Name(), Detail: detail}
		}
	}
	return Result{}
}
