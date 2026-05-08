package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(r.List()) != 1 {
		t.Error("should have 1 task")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil })
	if err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestRegistry_Trigger(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	called := 0
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error {
		called++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := r.Trigger(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}

	entry, _ := r.Get("test")
	if entry.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", entry.SuccessCount)
	}
}

func TestRegistry_TriggerNotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Trigger(context.Background(), "missing")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestRegistry_TriggerPaused(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.Pause("test"); err != nil {
		t.Fatal(err)
	}

	err := r.Trigger(context.Background(), "test")
	if err == nil {
		t.Error("expected error for paused task")
	}
}

func TestRegistry_TriggerError(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error {
		return errors.New("fail")
	}); err != nil {
		t.Fatal(err)
	}

	_ = r.Trigger(context.Background(), "test")

	entry, _ := r.Get("test")
	if entry.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", entry.ErrorCount)
	}
	if entry.LastError != "fail" {
		t.Errorf("expected 'fail', got %q", entry.LastError)
	}
}

func TestRegistry_PauseResume(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := r.Pause("test"); err != nil {
		t.Fatal(err)
	}
	entry, _ := r.Get("test")
	if !entry.Paused {
		t.Error("should be paused")
	}

	if err := r.Resume("test"); err != nil {
		t.Fatal(err)
	}
	entry, _ = r.Get("test")
	if entry.Paused {
		t.Error("should be resumed")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := r.Unregister("test"); err != nil {
		t.Fatal(err)
	}
	if len(r.List()) != 0 {
		t.Error("should have 0 tasks")
	}
}

func TestRegistry_PauseAll(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("t1", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("t2", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if err := r.PauseAll(); err != nil {
		t.Fatal(err)
	}
	for _, entry := range r.List() {
		if !entry.Paused {
			t.Errorf("task %s should be paused", entry.Name)
		}
	}
}

func TestNormalizeSchedule(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"*/5 * * * *", "0 */5 * * * *"},
		{"0 */2 * * *", "0 0 */2 * * *"},
		{"0 0 * * *", "0 0 0 * * *"},
		{"0 */6 * * *", "0 0 */6 * * *"},
		{"0 3 * * *", "0 0 3 * * *"},
		{"0 * * * * *", "0 * * * * *"},
		{"", "0 */5 * * * *"},
	}

	for _, tt := range tests {
		got := normalizeSchedule(tt.input)
		if got != tt.expect {
			t.Errorf("normalizeSchedule(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestRegistry_CronExecution(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	called := make(chan struct{}, 1)

	if err := r.Register("fast", "test", "* * * * * *", func(_ context.Context) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	if err := r.Trigger(ctx, "fast"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Error("handler should have been triggered")
	}
}

func TestRegistry_Reschedule(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	err := r.Reschedule("test", "*/10 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := r.Get("test")
	if entry.Schedule != "*/10 * * * *" {
		t.Errorf("expected */10, got %s", entry.Schedule)
	}
}

func TestRegistry_Reschedule_NotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Reschedule("missing", "*/5 * * * *")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

func TestRegistry_Reschedule_InvalidCron(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	if err := r.Register("test", "rss", "*/5 * * * *", func(_ context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	err := r.Reschedule("test", "invalid cron !@#")
	if err == nil {
		t.Error("expected error for invalid cron")
	}
}

func TestRegistry_Pause_NotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Pause("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Resume_NotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Resume("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Unregister_NotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Unregister("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	_, err := r.Get("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Register_InvalidSchedule(t *testing.T) {
	r := NewRegistry(zap.NewNop())
	err := r.Register("bad", "rss", "not-a-cron", func(_ context.Context) error { return nil })
	if err == nil {
		t.Error("expected error for invalid schedule")
	}
}

func TestNormalizeSchedule_EdgeCases(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"* * * * *", "0 * * * * *"},
		{"0 0 * * 1", "0 0 0 * * 1"},
	}
	for _, tt := range tests {
		got := normalizeSchedule(tt.input)
		if got != tt.expect {
			t.Errorf("normalizeSchedule(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}
