package db

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWriteQueue_Passthrough(t *testing.T) {
	db := newTestDB(t)
	q := NewPassthroughWriteQueue(db)
	q.Start(context.Background())

	called := atomic.Int32{}
	err := q.Execute(context.Background(), func(d *gorm.DB) error {
		called.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called.Load() != 1 {
		t.Fatal("fn not called")
	}
	q.Stop(context.Background())
}

func TestWriteQueue_Execute(t *testing.T) {
	db := newTestDB(t)
	q := NewWriteQueue(db, 16)
	q.Start(context.Background())
	defer q.Stop(context.Background())

	err := q.Execute(context.Background(), func(d *gorm.DB) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteQueue_ExecuteError(t *testing.T) {
	db := newTestDB(t)
	q := NewWriteQueue(db, 16)
	q.Start(context.Background())
	defer q.Stop(context.Background())

	err := q.Execute(context.Background(), func(d *gorm.DB) error {
		return errors.New("test error")
	})
	if err == nil || err.Error() != "test error" {
		t.Fatalf("expected test error, got %v", err)
	}
}

func TestWriteQueue_Cancelled(t *testing.T) {
	db := newTestDB(t)
	q := NewWriteQueue(db, 1)
	q.Start(context.Background())
	defer q.Stop(context.Background())

	fillReq := &writeRequest{fn: func(d *gorm.DB) error { time.Sleep(2 * time.Second); return nil }, resp: make(chan error, 1)}
	q.ch <- fillReq

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := q.Execute(ctx, func(d *gorm.DB) error { return nil })
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestWriteQueue_Stop(t *testing.T) {
	db := newTestDB(t)
	q := NewWriteQueue(db, 16)
	q.Start(context.Background())
	err := q.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
