package api

import (
	"encoding/json"
)

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// jsonRemap 把任意 payload 通过 marshal/unmarshal 转成目标类型
func jsonRemap(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
