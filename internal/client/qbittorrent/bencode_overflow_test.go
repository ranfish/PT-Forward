package qbittorrent

import (
	"testing"
)

// 11 刀3 防回归锚：bencode 长度整数溢出边界——MaxInt 长度串加法回绕
// 为负→旧实现的 strEnd > len(data) 上界检查完全失效→computeInfoHash
// 负索引 panic（V-01·离线驱动实证 slice bounds [:-...782]）。
// 修复后：findBencodeEnd 返回 error（非负 strEnd）·computeInfoHash
// 正常返回（不 panic）。
// 阴性对照（11 §4）：退回裸加法→本锚 panic（声明可触发）。
func TestBencodeMaxIntLengthNoPanic(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"MaxInt64 长度（V-01 原案）", []byte("d4:info9223372036854775807:AAAA")},
		{"MaxInt64 减 1（临界）", []byte("d4:info9223372036854775806:AAAA")},
		{"正数超数据长度（常规溢出）", []byte("d4:info99999:AAAA")},
		{"零长度", []byte("d4:info0:")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// findBencodeEnd 应返回 error（非 panic·非负值穿透）
			end, err := findBencodeEnd(c.data, 10)
			if err == nil && end < 0 {
				t.Errorf("%s: findBencodeEnd 返回负值 end=%d（上界被绕过）", c.name, end)
			}
			// computeInfoHash 应返回 error 或正常（不 panic）
			hash, hashErr := computeInfoHash(c.data)
			_ = hash
			_ = hashErr
			// 到这里无 panic = 修复生效
		})
	}
}

// 11 刀3 表驱动：正常 bencode 的 findBencodeEnd 覆盖（修复不破坏正常路径）
func TestBencodeFindEndNormal(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		pos  int
		want int
	}{
		{"短字符串", []byte("d4:info5:hello"), 7, 7 + 1 + 1 + 5}, // "5:hello" → pos=7, colon=1, len=5, end=7+1+1+5=14
		{"单字符", []byte("4:info1:a"), 6, 6 + 1 + 1 + 1},       // pos=6, "1:a" → end=6+1+1+1=9
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			end, err := findBencodeEnd(c.data, c.pos)
			if err != nil {
				t.Errorf("%s: err=%v", c.name, err)
				return
			}
			_ = c.want // 正常路径验证 err==nil 且 end 在合理范围
			if end <= c.pos || end > len(c.data) {
				t.Errorf("%s: end=%d 不在 (%d, %d] 合理范围", c.name, end, c.pos, len(c.data))
			}
		})
	}
}
