package db

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

type writeRequest struct {
	fn   func(*gorm.DB) error
	resp chan error
}

type WriteQueue struct {
	ch       chan *writeRequest
	db       *gorm.DB
	done     chan struct{}
	passMode bool
	wg       sync.WaitGroup
}

func NewWriteQueue(db *gorm.DB, bufSize int) *WriteQueue {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &WriteQueue{
		ch:   make(chan *writeRequest, bufSize),
		db:   db,
		done: make(chan struct{}),
	}
}

func NewPassthroughWriteQueue(db *gorm.DB) *WriteQueue {
	return &WriteQueue{
		db:       db,
		passMode: true,
		done:     make(chan struct{}),
	}
}

func (q *WriteQueue) Start(_ context.Context) {
	if q.passMode {
		return
	}
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		for req := range q.ch {
			err := req.fn(q.db)
			req.resp <- err
		}
		close(q.done)
	}()
}

func (q *WriteQueue) Stop(_ context.Context) error {
	if q.passMode {
		close(q.done)
		return nil
	}
	close(q.ch)
	q.wg.Wait()
	return nil
}

func (q *WriteQueue) Done() <-chan struct{} {
	return q.done
}

func (q *WriteQueue) Execute(ctx context.Context, fn func(*gorm.DB) error) error {
	if q.passMode {
		return fn(q.db)
	}

	req := &writeRequest{
		fn:   fn,
		resp: make(chan error, 1),
	}

	select {
	case q.ch <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err, ok := <-req.resp:
		if !ok {
			return fmt.Errorf("write queue stopped")
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
