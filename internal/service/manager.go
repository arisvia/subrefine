package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/arisvia/subrefine/internal/auth"
	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/core/parser"
	"github.com/arisvia/subrefine/internal/core/pipeline"
	"github.com/arisvia/subrefine/internal/storage"
)

var (
	ErrUnauthorized = errors.New("unauthorized subscription token")
	ErrDisabled     = errors.New("subscription is disabled")
)

type cacheItem struct {
	nodes     []*model.ProxyNode
	fetchedAt time.Time
}

type Manager struct {
	store   *storage.Store
	fetcher *fetcher.Fetcher
	cache   map[string]*cacheItem
	cacheMu sync.RWMutex
	ttl     time.Duration
}

func NewManager(store *storage.Store, f *fetcher.Fetcher, cacheTTL time.Duration) *Manager {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &Manager{
		store:   store,
		fetcher: f,
		cache:   make(map[string]*cacheItem),
		ttl:     cacheTTL,
	}
}

// GetSubscription fetches, pipelines, and returns the nodes for a subscription.
func (m *Manager) GetSubscription(id string, token string) ([]*model.ProxyNode, error) {
	sub, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}

	if !auth.ValidateSubToken(token, sub.Token) {
		return nil, ErrUnauthorized
	}

	if !sub.Enabled {
		return nil, ErrDisabled
	}

	// Check cache
	m.cacheMu.RLock()
	item, ok := m.cache[id]
	if ok && time.Since(item.fetchedAt) < m.ttl {
		m.cacheMu.RUnlock()
		return item.nodes, nil
	}
	m.cacheMu.RUnlock()

	// Fetch raw nodes from all sources
	var collected []*model.ProxyNode
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, src := range sub.Sources {
		nodes, err := m.fetchSource(ctx, src)
		if err == nil && len(nodes) > 0 {
			collected = append(collected, nodes...)
		}
	}

	// Apply pipeline operators
	processed := m.applyPipeline(collected, sub.Pipeline)

	// Update cache
	m.cacheMu.Lock()
	m.cache[id] = &cacheItem{
		nodes:     processed,
		fetchedAt: time.Now(),
	}
	m.cacheMu.Unlock()

	return processed, nil
}

func (m *Manager) fetchSource(ctx context.Context, src string) ([]*model.ProxyNode, error) {
	if len(src) >= 7 && (src[:7] == "http://" || src[:8] == "https://") {
		return m.fetcher.FetchSubscription(ctx, src, "")
	}
	// Try parsing raw URI directly
	node, err := parser.ParseURI(src)
	if err == nil && node != nil {
		return []*model.ProxyNode{node}, nil
	}
	return nil, fmt.Errorf("unrecognized source: %s", src)
}

func (m *Manager) applyPipeline(nodes []*model.ProxyNode, steps []storage.PipelineStep) []*model.ProxyNode {
	current := nodes
	for _, step := range steps {
		switch step.Type {
		case "rename":
			current = pipeline.RegexRename(current, step.Pattern, step.Repl)
		case "filter":
			current = pipeline.Filter(current, step.Include, step.Exclude)
		case "deduplicate":
			current = pipeline.Deduplicate(current)
		case "decorate":
			current = pipeline.Decorate(current, pipeline.DecoratorOptions{
				Fingerprint: step.Fingerprint,
				Detour:      step.Detour,
			})
		}
	}
	return current
}
