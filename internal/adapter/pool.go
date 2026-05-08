package adapter

import (
	"context"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type Pool struct {
	mu       sync.RWMutex
	adapters map[string]model.SiteAdapter
	factory  *Factory
	logger   *zap.Logger
}

func NewPool(factory *Factory, logger *zap.Logger) *Pool {
	return &Pool{
		adapters: make(map[string]model.SiteAdapter),
		factory:  factory,
		logger:   logger,
	}
}

func (p *Pool) Start(ctx context.Context) error {
	p.logger.Info("adapter pool started")
	return nil
}

func (p *Pool) Get(ctx context.Context, domain, framework string) (model.SiteAdapter, error) {
	key := domain + ":" + framework

	p.mu.RLock()
	if a, ok := p.adapters[key]; ok {
		p.mu.RUnlock()
		return a, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if a, ok := p.adapters[key]; ok {
		return a, nil
	}

	a := p.factory.Create(framework, NewHTTPDoer())
	p.adapters[key] = a
	p.logger.Info("adapter created", zap.String("domain", domain), zap.String("framework", framework))
	return a, nil
}

func (p *Pool) GetWithFramework(ctx context.Context, domain, framework string) (model.SiteAdapter, error) {
	key := domain + ":" + framework

	p.mu.RLock()
	if a, ok := p.adapters[key]; ok {
		p.mu.RUnlock()
		return a, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if a, ok := p.adapters[key]; ok {
		return a, nil
	}

	a := p.factory.Create(framework, NewHTTPDoer())
	p.adapters[key] = a
	p.logger.Info("adapter created", zap.String("domain", domain), zap.String("framework", framework))
	return a, nil
}

func (p *Pool) Rebuild(ctx context.Context, domain string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, a := range p.adapters {
		parts := strings.SplitN(k, ":", 2)
		if parts[0] == domain {
			delete(p.adapters, k)
			_ = a
		}
	}

	p.logger.Info("adapter pool rebuilt", zap.String("domain", domain))
	return nil
}

func (p *Pool) Remove(domain string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k := range p.adapters {
		parts := strings.SplitN(k, ":", 2)
		if parts[0] == domain {
			delete(p.adapters, k)
		}
	}
}

func (p *Pool) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := len(p.adapters)
	p.adapters = make(map[string]model.SiteAdapter)
	p.logger.Info("adapter pool closed", zap.Int("removed", count))
	return nil
}

func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.adapters)
}
