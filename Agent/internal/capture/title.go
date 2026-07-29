package capture

import (
	"bytes"
	"unicode/utf8"
)

// extractHTMLTitle 尝试从 HTTP 响应负载中提取 <title>...</title> 内容。
// 仅适用于明文 HTTP。HTTPS 因加密无法直接解析，需要配合 TLS MITM（暂不实现）。
func extractHTMLTitle(payload []byte) (string, bool) {
	// 必须是 HTTP 响应：以 HTTP/ 开头
	if !bytes.HasPrefix(payload, []byte("HTTP/")) {
		return "", false
	}
	// 找 <title
	lower := bytes.ToLower(payload)
	start := bytes.Index(lower, []byte("<title"))
	if start < 0 {
		return "", false
	}
	// 跳过 <title ...>
	gt := bytes.IndexByte(lower[start:], '>')
	if gt < 0 {
		return "", false
	}
	body := payload[start+gt+1:]
	end := bytes.Index(bytes.ToLower(body), []byte("</title>"))
	if end < 0 {
		return "", false
	}
	title := decodeEntities(trimSpaces(body[:end]))
	if !utf8.ValidString(title) {
		return "", false
	}
	if len(title) == 0 || len(title) > 256 {
		return "", false
	}
	return title, true
}

func trimSpaces(s []byte) []byte {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// decodeEntities 仅解码最常见的 HTML 实体，足够标题展示用途。
func decodeEntities(b []byte) string {
	s := string(b)
	repls := []struct{ from, to string }{
		{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&quot;", "\""}, {"&#39;", "'"},
		{"&nbsp;", " "},
	}
	for _, r := range repls {
		s = stringsReplaceAll(s, r.from, r.to)
	}
	return s
}

func stringsReplaceAll(s, old, new string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}
