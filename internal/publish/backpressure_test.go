package publish

import (
	"context"
	"testing"
)

func TestBackpressureController_AcquireRelease(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.MaxConcurrent = 2
	ctrl := NewBackpressureController(nil, cfg, nil)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := ctrl.AcquireSlot(context.TODO(), "site2"); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if err := ctrl.AcquireSlot(context.TODO(), "site3"); err != ErrBackpressureConcurrency {
		t.Fatalf("third acquire should fail with concurrency error, got: %v", err)
	}

	ctrl.ReleaseSlot("site1", true)
	if err := ctrl.AcquireSlot(context.TODO(), "site3"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	ctrl.ReleaseSlot("site2", true)
	ctrl.ReleaseSlot("site3", true)
}

func TestBackpressureController_SiteFailure(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.SiteConsecutiveFailures = 2
	cfg.SiteCooldownMinutes = 5
	ctrl := NewBackpressureController(nil, cfg, nil)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	ctrl.ReleaseSlot("site1", false)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	ctrl.ReleaseSlot("site1", false)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != ErrBackpressureSitePaused {
		t.Fatalf("third acquire should fail with site paused, got: %v", err)
	}
}

func TestBackpressureController_SiteRecovery(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.SiteConsecutiveFailures = 2
	ctrl := NewBackpressureController(nil, cfg, nil)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctrl.ReleaseSlot("site1", false)

	if err := ctrl.AcquireSlot(context.TODO(), "site1"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	ctrl.ReleaseSlot("site1", true)

	if ctrl.isSitePaused("site1") {
		t.Error("site should not be paused after success")
	}
}

func TestBackpressureController_DiskIO(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.DiskIOCheckEnabled = true
	cfg.DiskIOMaxConcurrent = 2
	ctrl := NewBackpressureController(nil, cfg, nil)

	if err := ctrl.AcquireDiskIOSlot(); err != nil {
		t.Fatalf("first disk IO: %v", err)
	}
	if err := ctrl.AcquireDiskIOSlot(); err != nil {
		t.Fatalf("second disk IO: %v", err)
	}
	if err := ctrl.AcquireDiskIOSlot(); err != ErrBackpressureDiskIO {
		t.Fatalf("third disk IO should fail, got: %v", err)
	}

	ctrl.ReleaseDiskIOSlot()
	if err := ctrl.AcquireDiskIOSlot(); err != nil {
		t.Fatalf("disk IO after release: %v", err)
	}
	ctrl.ReleaseDiskIOSlot()
	ctrl.ReleaseDiskIOSlot()
}

func TestBackpressureController_DiskIODisabled(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.DiskIOCheckEnabled = false
	ctrl := NewBackpressureController(nil, cfg, nil)

	for i := 0; i < 100; i++ {
		if err := ctrl.AcquireDiskIOSlot(); err != nil {
			t.Fatalf("disk IO should always succeed when disabled: %v", err)
		}
	}
}

func TestBackpressureController_Status(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.MaxConcurrent = 5
	ctrl := NewBackpressureController(nil, cfg, nil)

	_ = ctrl.AcquireSlot(context.TODO(), "site1")
	_ = ctrl.AcquireSlot(context.TODO(), "site2")

	status := ctrl.Status()
	if status.ActivePublishes != 2 {
		t.Errorf("expected 2 active, got %d", status.ActivePublishes)
	}
	if status.MaxConcurrentPublishes != 5 {
		t.Errorf("expected max 5, got %d", status.MaxConcurrentPublishes)
	}
	if status.IsThrottled {
		t.Error("should not be throttled at 2/5")
	}

	ctrl.ReleaseSlot("site1", true)
	ctrl.ReleaseSlot("site2", true)
}

func TestBackpressureController_CleanupExpired(t *testing.T) {
	cfg := DefaultBackpressureConfig()
	cfg.SiteConsecutiveFailures = 1
	cfg.SiteCooldownMinutes = 0
	ctrl := NewBackpressureController(nil, cfg, nil)

	_ = ctrl.AcquireSlot(context.TODO(), "site1")
	ctrl.ReleaseSlot("site1", false)

	ctrl.cleanupExpiredPauses()
}
