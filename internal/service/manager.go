package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	// def pins the cache to the exact subscription definition it was built
	// from, so editing sources or the pipeline takes effect on the next request
	// instead of waiting out the TTL.
	def string
}

func fingerprint(sub *storage.Subscription) string {
	data, err := json.Marshal(sub)
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return string(data)
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
	fp := fingerprint(sub)
	m.cacheMu.RLock()
	item, ok := m.cache[id]
	if ok && item.def == fp && time.Since(item.fetchedAt) < m.ttl {
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

	// Update cache, dropping entries that are no longer reachable or usable.
	m.cacheMu.Lock()
	for key, it := range m.cache {
		if time.Since(it.fetchedAt) > m.ttl {
			delete(m.cache, key)
		}
	}
	m.cache[id] = &cacheItem{
		nodes:     processed,
		fetchedAt: time.Now(),
		def:       fp,
	}
	m.cacheMu.Unlock()

	return processed, nil
}

func (m *Manager) fetchSource(ctx context.Context, src string) ([]*model.ProxyNode, error) {
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		return m.fetcher.FetchSubscription(ctx, src, "")
	default:
		// Inline payload: single URI, base64 blob, Clash YAML or sing-box JSON fragment.
		nodes, err := parser.ParseSubscription([]byte(src))
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
		return nil, fmt.Errorf("unrecognized source: %s", src)
	}
}

func (m *Manager) applyPipeline(nodes []*model.ProxyNode, steps []storage.PipelineStep) []*model.ProxyNode {
	current := nodes
	for _, step := range steps {
		switch step.Type {
		case "rename":
			current = pipeline.RegexRename(current, step.Pattern, step.Repl)
		case "filter":
			current = pipeline.Filter(current, step.Include, step.Exclude, step.Types...)
		case "deduplicate":
			current = pipeline.Deduplicate(current)
		case "sort":
			current = pipeline.Sort(current, step.By, step.Order)
		case "decorate":
			current = pipeline.Decorate(current, pipeline.DecoratorOptions{
				Fingerprint: step.Fingerprint,
				Detour:      step.Detour,
			})
		}
	}
	return current
}
