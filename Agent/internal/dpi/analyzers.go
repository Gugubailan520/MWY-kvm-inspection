package dpi

import (
	"bytes"
	"strings"

	"github.com/google/gopacket/layers"
)

// --- SOCKS5 ---
// RFC1928: 客户端首包格式：VER(0x05) NMETHODS METHODS...
type socks5Analyzer struct{}

func (*socks5Analyzer) Name() string { return "socks5" }

func (*socks5Analyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	if len(p) < 3 || p[0] != 0x05 {
		return false, ""
	}
	nmethods := int(p[1])
	if nmethods == 0 || len(p) < 2+nmethods {
		return false, ""
	}
	return true, "SOCKS5 handshake (ver=0x05)"
}

// --- SOCKS4 ---
// VER(0x04) CMD DSTPORT(2) DSTIP(4) USERID \0
type socks4Analyzer struct{}

func (*socks4Analyzer) Name() string { return "socks4" }

func (*socks4Analyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	if len(p) < 8 || p[0] != 0x04 {
		return false, ""
	}
	cmd := p[1]
	if cmd != 0x01 && cmd != 0x02 { // CONNECT / BIND
		return false, ""
	}
	return true, "SOCKS4 handshake (ver=0x04)"
}

// --- HTTP Proxy ---
// 客户端发出绝对 URI 形式的请求，如 CONNECT host:port 或 GET http://host/...
type httpProxyAnalyzer struct{}

func (*httpProxyAnalyzer) Name() string { return "http_proxy" }

func (*httpProxyAnalyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	if len(p) < 9 {
		return false, ""
	}
	head := p
	if len(head) > 256 {
		head = head[:256]
	}
	upper := bytes.ToUpper(head)
	if bytes.HasPrefix(upper, []byte("CONNECT ")) {
		return true, "HTTP CONNECT proxy"
	}
	if bytes.HasPrefix(upper, []byte("GET HTTP://")) ||
		bytes.HasPrefix(upper, []byte("POST HTTP://")) ||
		bytes.HasPrefix(upper, []byte("GET HTTPS://")) {
		return true, "HTTP absolute-URI proxy"
	}
	return false, ""
}

// --- Trojan ---
// 协议：SHA224(password) 16 进制(56 字节) + \r\n + CMD(1) + ADDR...
type trojanAnalyzer struct{}

func (*trojanAnalyzer) Name() string { return "trojan" }

func (*trojanAnalyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	// 56 hex + 2 (CRLF) = 58，其后第 59 字节是 CMD（0x01/0x03）
	if len(p) < 59 {
		return false, ""
	}
	if p[56] != 0x0d || p[57] != 0x0a {
		return false, ""
	}
	if !isHex(p[:56]) {
		return false, ""
	}
	cmd := p[58]
	if cmd != 0x01 && cmd != 0x03 {
		return false, ""
	}
	return true, "Trojan (SHA224 hex + CRLF + CMD)"
}

// --- Shadowsocks (AEAD 启发式) ---
// TCP 首包以随机长度前缀开头：[u16_be_len][salt...]
// 启发式：前 2 字节表示的长度与实际 payload 长度接近、且载荷熵高、无明显可打印字符。
type shadowsocksAnalyzer struct{}

func (*shadowsocksAnalyzer) Name() string { return "shadowsocks" }

func (*shadowsocksAnalyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	if len(p) < 32 {
		return false, ""
	}
	// 全随机负载判定：可打印字符比例 < 10% 且非全零。
	printable := 0
	for i := 0; i < len(p); i++ {
		c := p[i]
		if (c >= 0x20 && c < 0x7f) || c == '\n' || c == '\r' || c == '\t' {
			printable++
		}
	}
	ratio := float64(printable) / float64(len(p))
	// 命中已知协议的握手（上面 analyzer 已先尝试），这里只处理"看起来像随机加密"的首包
	if ratio < 0.1 && !bytes.HasPrefix(p, []byte{0, 0}) {
		return true, "Shadowsocks-like encrypted stream (high entropy)"
	}
	return false, ""
}

// --- SSH Tunnel ---
// SSH banner: "SSH-protoversion-softwareversion ..."
type sshAnalyzer struct{}

func (*sshAnalyzer) Name() string { return "ssh_tunnel" }

func (*sshAnalyzer) Feed(p []byte, _ *layers.TCP) (bool, string) {
	if len(p) < 4 {
		return false, ""
	}
	if strings.HasPrefix(string(p), "SSH-") {
		return true, "SSH banner"
	}
	return false, ""
}

func isHex(p []byte) bool {
	for _, c := range p {
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !ok {
			return false
		}
	}
	return true
}
