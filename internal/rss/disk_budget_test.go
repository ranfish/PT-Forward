package rss

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDiskBudget_ReserveSuccess(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	ticket, err := m.Reserve("client1", 1000, 5000, 5*time.Minute)
	if err != nil {
		t.Fatalf("reserve should succeed: %v", err)
	}
	if ticket == nil {
		t.Fatal("ticket should not be nil")
	}
	if ticket.Size != 1000 {
		t.Errorf("ticket size = %d, want 1000", ticket.Size)
	}
}

func TestDiskBudget_ReserveInsufficient(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	_, err := m.Reserve("client1", 5000, 3000, 5*time.Minute)
	if err == nil {
		t.Fatal("reserve should fail when size > freeSpace")
	}
}

func TestDiskBudget_ReserveMultiple(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	_, err := m.Reserve("client1", 3000, 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}

	_, err = m.Reserve("client1", 3000, 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}

	_, err = m.Reserve("client1", 5000, 10000, 5*time.Minute)
	if err == nil {
		t.Fatal("third reserve should fail: over budget (3000+3000+5000 > 10000)")
	}
}

func TestDiskBudget_Release(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	ticket, _ := m.Reserve("client1", 8000, 10000, 5*time.Minute)
	m.Release(ticket)

	_, err := m.Reserve("client1", 8000, 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("after release, reserve should succeed: %v", err)
	}
}

func TestDiskBudget_ReleaseNil(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())
	m.Release(nil)
}

func TestDiskBudget_Expire(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	_, _ = m.Reserve("client1", 8000, 10000, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	m.Expire()

	reserved := m.ReservedBytes("client1")
	if reserved != 0 {
		t.Errorf("after expire, reserved = %d, want 0", reserved)
	}
}

func TestDiskBudget_DifferentClients(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	_, err := m.Reserve("client1", 8000, 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("client1 reserve: %v", err)
	}

	_, err = m.Reserve("client2", 8000, 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("client2 reserve should be independent: %v", err)
	}
}

func TestDiskBudget_ReservedBytes(t *testing.T) {
	m := NewDiskBudgetManager(zap.NewNop())

	_, _ = m.Reserve("client1", 3000, 10000, 5*time.Minute)
	_, _ = m.Reserve("client1", 2000, 10000, 5*time.Minute)

	if got := m.ReservedBytes("client1"); got != 5000 {
		t.Errorf("ReservedBytes(client1) = %d, want 5000", got)
	}
	if got := m.ReservedBytes("client2"); got != 0 {
		t.Errorf("ReservedBytes(client2) = %d, want 0", got)
	}
}
