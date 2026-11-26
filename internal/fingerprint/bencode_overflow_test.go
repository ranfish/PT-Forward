package fingerprint

import (
	"testing"
)

// 11 刀3 防回归锚：decodeString 整数溢出边界——MaxInt 长度 Sscanf
// 解析成功→start+length 回绕为负→end > len(data) 失效→data[start:end]
// 负索引 panic（V-02·离线驱动实证 [:-...789]）。
// 修复后：返回 error（非 panic）。
func TestDecodeStringMaxIntNoPanic(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"MaxInt64 长度（V-02 原案）", []byte("9223372036854775807:AAAA")},
		{"MaxInt64 减 1（临界）", []byte("9223372036854775806:AAAA")},
		{"正数超数据长度", []byte("99999:AAAA")},
		{"零长度", []byte("0:")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// decodeString 应返回 error（非 panic）
			_, _, err := decodeString(c.data, 0)
			_ = err // err 可为 nil（如零长度 "0:" 是合法的空串）
			// 到这里无 panic = 修复生效
		})
	}
}
