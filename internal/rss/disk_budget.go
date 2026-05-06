package rss

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type DiskBudgetTicket struct {
	ID        string
	ClientID  string
	Size      int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type DiskBudgetManager struct {
	mu      sync.Mutex
	tickets map[string]*DiskBudgetTicket
	logger  *zap.Logger
}

func NewDiskBudgetManager(logger *zap.Logger) *DiskBudgetManager {
	return &DiskBudgetManager{
		tickets: make(map[string]*DiskBudgetTicket),
		logger:  logger,
	}
}

func (m *DiskBudgetManager) Reserve(clientID string, size int64, freeSpace int64, ttl time.Duration) (*DiskBudgetTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var reserved int64
	for _, t := range m.tickets {
		if t.ClientID == clientID && time.Now().Before(t.ExpiresAt) {
			reserved += t.Size
		}
	}

	available := freeSpace - reserved
	if available < size {
		return nil, rssError(ErrRSSDisk, fmt.Sprintf("磁盘预算不足: 可用 %d 字节 (已预留 %d), 需要 %d 字节", available, reserved, size), nil)
	}

	ticket := &DiskBudgetTicket{
		ID:        fmt.Sprintf("ticket-%d", time.Now().UnixNano()),
		ClientID:  clientID,
		Size:      size,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	m.tickets[ticket.ID] = ticket

	m.logger.Debug("磁盘预算预留",
		zap.String("ticket", ticket.ID),
		zap.String("client", clientID),
		zap.Int64("size", size),
		zap.Int64("reserved", reserved+size),
		zap.Int64("freeSpace", freeSpace),
	)

	return ticket, nil
}

func (m *DiskBudgetManager) Release(ticket *DiskBudgetTicket) {
	if ticket == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tickets[ticket.ID]; ok {
		delete(m.tickets, ticket.ID)
		m.logger.Debug("磁盘预算释放",
			zap.String("ticket", ticket.ID),
			zap.String("client", ticket.ClientID),
			zap.Int64("size", ticket.Size),
		)
	}
}

func (m *DiskBudgetManager) Commit(ticket *DiskBudgetTicket) {
	if ticket == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tickets, ticket.ID)
}

func (m *DiskBudgetManager) Expire() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expired := 0
	for id, t := range m.tickets {
		if now.After(t.ExpiresAt) {
			delete(m.tickets, id)
			expired++
		}
	}
	if expired > 0 {
		m.logger.Debug("磁盘预算过期清理", zap.Int("expired", expired))
	}
}

func (m *DiskBudgetManager) ReservedBytes(clientID string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	var total int64
	now := time.Now()
	for _, t := range m.tickets {
		if t.ClientID == clientID && now.Before(t.ExpiresAt) {
			total += t.Size
		}
	}
	return total
}
