package audit

import (
	"context"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Logger struct {
	db     *gorm.DB
	logger *zap.Logger
	ch     chan *model.OperationAuditLog
	wg     sync.WaitGroup
}

func NewLogger(db *gorm.DB, logger *zap.Logger) *Logger {
	return &Logger{
		db:     db,
		logger: logger,
		ch:     make(chan *model.OperationAuditLog, 1000),
	}
}

func (l *Logger) Start(ctx context.Context) {
	l.wg.Add(1)
	go l.flushLoop(ctx)
}

func (l *Logger) Stop() {
	l.wg.Wait()
}

func (l *Logger) Log(actor, module, action, targetType, targetID, detail, result string) {
	entry := &model.OperationAuditLog{
		Actor:      actor,
		Module:     module,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		Result:     result,
	}

	select {
	case l.ch <- entry:
	default:
		l.logger.Warn("audit log channel full, dropping entry",
			zap.String("action", action),
			zap.String("target", targetID),
		)
	}
}

func (l *Logger) flushLoop(ctx context.Context) {
	defer l.wg.Done()
	batch := make([]*model.OperationAuditLog, 0, 50)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case entry := <-l.ch:
					batch = append(batch, entry)
				default:
					l.flush(batch)
					return
				}
			}
		case entry := <-l.ch:
			batch = append(batch, entry)
			if len(batch) >= 50 {
				l.flush(batch)
				batch = make([]*model.OperationAuditLog, 0, 50)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flush(batch)
				batch = make([]*model.OperationAuditLog, 0, 50)
			}
		}
	}
}

func (l *Logger) flush(batch []*model.OperationAuditLog) {
	if len(batch) == 0 {
		return
	}
	if err := l.db.CreateInBatches(batch, 50).Error; err != nil {
		l.logger.Error("flush audit logs failed", zap.Error(err), zap.Int("count", len(batch)))
	}
}
