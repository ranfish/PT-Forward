package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type TaskFunc func(ctx context.Context) error

type TaskEntry struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Schedule     string     `json:"schedule"`
	Handler      TaskFunc   `json:"-"`
	LastRunAt    *time.Time `json:"last_run_at"`
	LastError    string     `json:"last_error"`
	SuccessCount int64      `json:"success_count"`
	ErrorCount   int64      `json:"error_count"`
	Paused       bool       `json:"paused"`
	Running      bool       `json:"running"`
}

type Registry struct {
	mu      sync.RWMutex
	statsMu sync.Mutex
	tasks   map[string]*TaskEntry
	logger  *zap.Logger
	cron    *cron.Cron
	ids     map[string]cron.EntryID
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		tasks:  make(map[string]*TaskEntry),
		logger: logger,
		cron:   cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		ids:    make(map[string]cron.EntryID),
	}
}

func (r *Registry) Register(name, taskType, schedule string, handler TaskFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[name]; exists {
		return schedulerError(ErrSchedulerDuplicate, fmt.Sprintf("task %q already registered", name), nil)
	}

	entry := &TaskEntry{
		Name:     name,
		Type:     taskType,
		Schedule: schedule,
		Handler:  handler,
	}

	r.tasks[name] = entry

	scheduleStr := normalizeSchedule(schedule)
	id, err := r.cron.AddFunc(scheduleStr, func() {
		r.mu.RLock()
		paused := entry.Paused
		r.mu.RUnlock()
		if paused {
			return
		}
		r.runTask(context.Background(), entry) //nolint:gosec,errcheck // runTask handles errors internally
	})

	if err != nil {
		delete(r.tasks, name)
		return schedulerError(ErrSchedulerSchedule, fmt.Sprintf("invalid cron schedule %q", scheduleStr), err)
	}

	r.ids[name] = id

	return nil
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[name]; !exists {
		return schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}

	if id, ok := r.ids[name]; ok {
		r.cron.Remove(id)
		delete(r.ids, name)
	}
	delete(r.tasks, name)
	return nil
}

func (r *Registry) Trigger(ctx context.Context, name string) error {
	r.mu.RLock()
	entry, exists := r.tasks[name]
	r.mu.RUnlock()

	if !exists {
		return schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}

	if entry.Paused {
		return schedulerError(ErrSchedulerPaused, fmt.Sprintf("task %q is paused", name), nil)
	}

	return r.runTask(ctx, entry)
}

func (r *Registry) Pause(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.tasks[name]
	if !exists {
		return schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}
	entry.Paused = true
	return nil
}

func (r *Registry) Resume(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.tasks[name]
	if !exists {
		return schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}
	entry.Paused = false
	return nil
}

func (r *Registry) Reschedule(name, schedule string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.tasks[name]
	if !exists {
		return schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}

	scheduleStr := normalizeSchedule(schedule)
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(scheduleStr); err != nil {
		return schedulerError(ErrSchedulerSchedule, fmt.Sprintf("invalid cron schedule %q", schedule), err)
	}

	if oldID, ok := r.ids[name]; ok {
		r.cron.Remove(oldID)
	}

	newID, err := r.cron.AddFunc(scheduleStr, func() {
		r.mu.RLock()
		paused := entry.Paused
		r.mu.RUnlock()
		if paused {
			return
		}
		r.runTask(context.Background(), entry) //nolint:gosec,errcheck // runTask handles errors internally
	})

	if err != nil {
		return schedulerError(ErrSchedulerSchedule, fmt.Sprintf("failed to reschedule %q", name), err)
	}

	r.ids[name] = newID
	entry.Schedule = schedule
	return nil
}

func (r *Registry) PauseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.tasks {
		entry.Paused = true
	}
	return nil
}

func (r *Registry) List() []*TaskEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*TaskEntry, 0, len(r.tasks))
	for _, entry := range r.tasks {
		result = append(result, entry)
	}
	return result
}

func (r *Registry) Get(name string) (*TaskEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.tasks[name]
	if !exists {
		return nil, schedulerError(ErrSchedulerNotFound, fmt.Sprintf("task %q not found", name), nil)
	}
	return entry, nil
}

func (r *Registry) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	r.cron.Start()

	r.mu.RLock()
	r.logger.Info("task registry started", zap.Int("tasks", len(r.tasks)))
	r.mu.RUnlock()

	return nil
}

func (r *Registry) Stop() error {
	r.mu.Lock()
	r.logger.Info("task registry stopping: draining in-flight tasks")
	r.mu.Unlock()

	stopCtx := r.cron.Stop()

	drainTimer := time.NewTimer(30 * time.Second)
	defer drainTimer.Stop()

	select {
	case <-stopCtx.Done():
		r.mu.Lock()
		r.logger.Info("all in-flight tasks drained")
		r.mu.Unlock()
	case <-drainTimer.C:
		r.mu.Lock()
		r.logger.Warn("drain timeout exceeded, forcing stop")
		r.mu.Unlock()
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.mu.Lock()
	r.logger.Info("task registry stopped")
	r.mu.Unlock()
	return nil
}

func (r *Registry) runTask(ctx context.Context, entry *TaskEntry) error {
	r.statsMu.Lock()
	if entry.Running {
		r.statsMu.Unlock()
		r.logger.Warn("task already running, skipping", zap.String("task", entry.Name))
		return nil
	}
	entry.Running = true
	r.statsMu.Unlock()

	now := time.Now()
	var err error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("task panicked",
					zap.String("task", entry.Name),
					zap.Any("recover", rec),
				)
				err = fmt.Errorf("panic: %v", rec)
			}
		}()
		err = entry.Handler(ctx)
	}()

	r.statsMu.Lock()
	entry.LastRunAt = &now
	entry.Running = false
	if err != nil {
		entry.LastError = err.Error()
		entry.ErrorCount++
	} else {
		entry.LastError = ""
		entry.SuccessCount++
	}
	r.statsMu.Unlock()

	return err
}

func normalizeSchedule(s string) string {
	switch len(s) {
	case 9:
		if s[0] == '*' && s[1] == '/' {
			return fmt.Sprintf("0 %s * * * *", s)
		}
	case 0:
		return "0 */5 * * * *"
	}

	parts := countFields(s)
	switch {
	case parts == 5:
		return "0 " + s
	case parts == 6:
		return s
	case parts == 1 && s == "*":
		return "0 * * * * *"
	}

	return "0 " + s
}

func countFields(s string) int {
	n := 0
	inField := false
	for _, c := range s {
		if c == ' ' || c == '\t' {
			inField = false
		} else if !inField {
			inField = true
			n++
		}
	}
	return n
}
