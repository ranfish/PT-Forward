package dispatcher

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	if cb == nil {
		t.Fatal("expected non-nil CircuitBreaker")
	}
	if cb.threshold != 5 {
		t.Errorf("default threshold should be 5, got %d", cb.threshold)
	}
}

func TestCircuitBreaker_Allow_ByDefault(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	if !cb.Allow("test-key") {
		t.Error("should allow by default")
	}
}

func TestCircuitBreaker_RecordFailure_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	cb.resetTimeout = 1 * time.Hour

	for i := 0; i < 5; i++ {
		cb.RecordFailure("test-key")
	}

	if cb.Allow("test-key") {
		t.Error("should block after reaching threshold")
	}
}

func TestCircuitBreaker_Allow_OtherKeysNotAffected(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	cb.resetTimeout = 1 * time.Hour

	for i := 0; i < 5; i++ {
		cb.RecordFailure("key-a")
	}

	if cb.Allow("key-a") {
		t.Error("key-a should be blocked")
	}
	if !cb.Allow("key-b") {
		t.Error("key-b should still be allowed")
	}
}

func TestCircuitBreaker_RecordSuccess_Resets(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	cb.resetTimeout = 1 * time.Hour

	for i := 0; i < 5; i++ {
		cb.RecordFailure("test-key")
	}

	cb.RecordSuccess("test-key")

	if !cb.Allow("test-key") {
		t.Error("should allow after success reset")
	}
}

func TestCircuitBreaker_Allow_AfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	cb.resetTimeout = 10 * time.Millisecond

	for i := 0; i < 5; i++ {
		cb.RecordFailure("test-key")
	}

	if cb.Allow("test-key") {
		t.Error("should be blocked immediately after opening")
	}

	time.Sleep(20 * time.Millisecond)

	if !cb.Allow("test-key") {
		t.Error("should allow after timeout expires")
	}
}

func TestCircuitBreaker_BelowThreshold(t *testing.T) {
	cb := NewCircuitBreaker(zap.NewNop(), nil)
	cb.resetTimeout = 1 * time.Hour

	for i := 0; i < 4; i++ {
		cb.RecordFailure("test-key")
	}

	if !cb.Allow("test-key") {
		t.Error("should still allow below threshold of 5")
	}
}
