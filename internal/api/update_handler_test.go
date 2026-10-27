package api

import "testing"

// TestOTASingleFlight §59.358: 单飞互斥——CAS 门在已置位时拒绝二进。
func TestOTASingleFlight(t *testing.T) {
	// 模拟进行中
	otaInProgress.Store(true)
	defer otaInProgress.Store(false)
	if otaInProgress.CompareAndSwap(false, true) {
		t.Fatal("已进行中 CAS 应失败（单飞门失效）")
	}
	// 重置后可进
	otaInProgress.Store(false)
	if !otaInProgress.CompareAndSwap(false, true) {
		t.Fatal("空闲态 CAS 应成功")
	}
}
