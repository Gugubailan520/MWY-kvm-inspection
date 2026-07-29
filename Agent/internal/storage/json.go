package storage

import (
	"encoding/json"

	"github.com/kvm-inspection/common"
)

// 为避免每个包各自实现，这里集中放置 JSON 序列化辅助。

func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonUnmarshal(s string) (*common.NetworkEvent, error) {
	var ev common.NetworkEvent
	if err := json.Unmarshal([]byte(s), &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}
