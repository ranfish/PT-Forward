package seeding

import (
	"testing"
	"time"
)

func TestFitTimer_MarkMatched(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	ft.MarkMatched(1, "h1", now)
	ft.MarkMatched(1, "h2", now)

	matched := ft.GetMatchedAt(1, "h1")
	if matched == nil {
		t.Fatal("expected h1 to be marked")
	}
	if matched.Sub(now) > time.Second {
		t.Errorf("expected matchedAt ~= now, got %v", matched)
	}
}

func TestFitTimer_MarkMatched_Idempotent(t *testing.T) {
	ft := NewFitTimer()
	t1 := time.Now()
	ft.MarkMatched(1, "h1", t1)

	t2 := t1.Add(10 * time.Second)
	ft.MarkMatched(1, "h1", t2)

	matched := ft.GetMatchedAt(1, "h1")
	if matched.Sub(t1) > time.Second {
		t.Errorf("expected first mark time preserved, got %v", matched)
	}
}

func TestFitTimer_IsFit(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	ft.MarkMatched(1, "h1", now.Add(-5*time.Second))

	if !ft.IsFit(1, "h1", 3, now) {
		t.Error("expected fit (5s elapsed >= 3s fitTime)")
	}
	if !ft.IsFit(1, "h1", 5, now) {
		t.Error("expected fit (5s elapsed >= 5s fitTime)")
	}
	if ft.IsFit(1, "h1", 10, now) {
		t.Error("expected not fit (5s elapsed < 10s fitTime)")
	}
	if !ft.IsFit(1, "h1", 0, now) {
		t.Error("expected fit when fitTime=0 (disabled)")
	}
}

func TestFitTimer_IsFit_NotMarked(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	if ft.IsFit(1, "h1", 10, now) {
		t.Error("expected not fit when never marked")
	}
}

func TestFitTimer_ClearUnmatched(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	ft.MarkMatched(1, "h1", now)
	ft.MarkMatched(1, "h2", now)
	ft.MarkMatched(1, "h3", now)

	active := map[string]bool{"h1": true, "h3": true}
	ft.ClearUnmatched(1, active)

	if ft.GetMatchedAt(1, "h2") != nil {
		t.Error("expected h2 to be cleared")
	}
	if ft.GetMatchedAt(1, "h1") == nil {
		t.Error("expected h1 to remain")
	}
}

func TestFitTimer_Remove(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	ft.MarkMatched(1, "h1", now)
	ft.MarkMatched(2, "h1", now)

	ft.Remove("h1")

	if ft.GetMatchedAt(1, "h1") != nil {
		t.Error("expected h1 removed for rule 1")
	}
	if ft.GetMatchedAt(2, "h1") != nil {
		t.Error("expected h1 removed for rule 2")
	}
}

func TestFitTimer_ClearUnmatched_DifferentRules(t *testing.T) {
	ft := NewFitTimer()
	now := time.Now()

	ft.MarkMatched(1, "h1", now)
	ft.MarkMatched(2, "h1", now)

	active := map[string]bool{}
	ft.ClearUnmatched(1, active)

	if ft.GetMatchedAt(1, "h1") != nil {
		t.Error("expected h1 cleared for rule 1")
	}
	if ft.GetMatchedAt(2, "h1") == nil {
		t.Error("expected h1 kept for rule 2")
	}
}
