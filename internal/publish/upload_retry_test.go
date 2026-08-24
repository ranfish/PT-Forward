package publish

import (
	"context"
	"errors"
	"testing"
)

// §59.61 附3: 上传三层修复的单元锚定（uploadWithRetry 行为）
type fakeUploader struct {
	calls    int
	failN    int // 前 N 次失败
	failAll  bool
	lastData []byte
}

func (f *fakeUploader) upload(ctx context.Context, data []byte, name string) (string, error) {
	f.calls++
	if f.failAll || f.calls <= f.failN {
		return "", errors.New("pixhost upload failed")
	}
	return "https://img3.pixhost.cc/ok.jpg", nil
}

func TestUploadWithRetry_SuccessAfterRetry(t *testing.T) {
	f := &fakeUploader{failN: 2}
	u, err := uploadWithRetry(context.Background(), f.upload, []byte("x"), "a.jpg", 3)
	if err != nil || u != "https://img3.pixhost.cc/ok.jpg" {
		t.Fatalf("两次失败后第三次应成功: %v %v", u, err)
	}
	if f.calls != 3 {
		t.Errorf("应重试至成功: calls=%d", f.calls)
	}
}

func TestUploadWithRetry_Exhausted(t *testing.T) {
	f := &fakeUploader{failAll: true}
	_, err := uploadWithRetry(context.Background(), f.upload, []byte("x"), "a.jpg", 2)
	if err == nil {
		t.Fatal("全失败应报错")
	}
	if f.calls != 2 {
		t.Errorf("重试上限: calls=%d", f.calls)
	}
}
