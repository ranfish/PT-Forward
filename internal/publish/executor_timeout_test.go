package publish

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

// §59.470: 上传超时判定锚——超时类错误触发站方复查通道（超时≠失败）。
func TestIsUploadTimeoutErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ctx DeadlineExceeded", context.DeadlineExceeded, true},
		{"wrapped DeadlineExceeded", fmt.Errorf("上传请求失败: %w", context.DeadlineExceeded), true},
		{"Client.Timeout 形态（南洋案实录）", errors.New(`上传请求失败: Post "https://nanyangpt.com/takeupload.php": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`), true},
		{"net timeout", &net.OpError{Err: errors.New("i/o timeout")}, true},
		{"非超时错误", errors.New("上传失败: 站方拒绝"), false},
		{"403", errors.New("status 403"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isUploadTimeoutErr(c.err); got != c.want {
				t.Errorf("isUploadTimeoutErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
