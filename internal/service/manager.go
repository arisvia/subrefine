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
	// info is the merged account metadata (expiry/traffic) of the same fetch;
	// sources keeps each upstream's own numbers for the console.
	info    *model.SubscriptionInfo
	sources []*model.SourceInfo
}

// fingerprint covers ONLY the node-affecting definition. Key edits (names,
// tokens, disabled lists) must not force an upstream re-fetch - they filter
// the cached node set at request time instead.
func fingerprint(sub *storage.Subscription) string {
	data, err := json.Marshal(struct {
		Sources  []string               `json:"sources"`
		Pipeline []storage.PipelineStep `json:"pipeline"`
	}{sub.Sources, sub.Pipeline})
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
	// flights coalesces concurrent misses for the same subscription, so a
	// hundred clients arriving as the TTL lapses trigger one upstream fetch.
	flights  map[string]*fetchCall
	flightMu sync.Mutex
}

// fetchCall is one in-flight build of a subscription's nodes.
type fetchCall struct {
	done  chan struct{}
	nodes []*model.ProxyNode
	err   error
}

func NewManager(store *storage.Store, f *fetcher.Fetcher, cacheTTL time.Duration) *Manager {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &Manager{
		store:   store,
		fetcher: f,
		cache:   make(map[string]*cacheItem),
		flights: make(map[string]*fetchCall),
		ttl:     cacheTTL,
	}
}

// FullModes reports the per-subscription full-config template modes
// (sing-box, clash). Missing or unreadable subscriptions read as empty:
// the fragment target is always the safe default.
func (m *Manager) FullModes(id string) (singbox, clash string) {
	sub, err := m.store.Get(id)
	if err != nil {
		return "", ""
	}
	return sub.SingboxMode, sub.ClashMode
}

// GetSubscription fetches, pipelines, and returns the full node set for a
// subscription, authenticating the token against the global delivery-key pool.
// Callers that miss the cache wait on whoever else is already building the same
// subscription instead of re-fetching every upstream source.
func (m *Manager) GetSubscription(id string, token string) ([]*model.ProxyNode, error) {
	sub, _, _, err := m.store.Authenticate(id, token)
	if err != nil {
		return nil, translateAuth(err)
	}
	if !sub.Enabled {
		return nil, ErrDisabled
	}
	return m.nodes(sub)
}

// Resolve authenticates the token against the global key pool, checks that the
// subscription attaches it, and returns the (subscription, key) pair's
// reference plus the full (unfiltered) node set. The caller applies
// ref.Disabled per request, so per-key blacklists never poison or re-fetch the
// shared upstream cache.
func (m *Manager) Resolve(id, token string) (*storage.Subscription, *storage.SubKeyRef, []*model.ProxyNode, error) {
	sub, ref, err := m.auth(id, token)
	if err != nil {
		return nil, nil, nil, err
	}
	nodes, err := m.nodes(sub)
	if err != nil {
		return sub, ref, nil, err
	}
	return sub, ref, nodes, nil
}

func (m *Manager) auth(id, token string) (*storage.Subscription, *storage.SubKeyRef, error) {
	sub, _, ref, err := m.store.Authenticate(id, token)
	if err != nil {
		return nil, nil, translateAuth(err)
	}
	if !sub.Enabled {
		return nil, nil, ErrDisabled
	}
	return sub, ref, nil
}

// translateAuth maps store auth failures onto the service error vocabulary so
// the API can answer 403 without inspecting storage internals. Unknown ids
// collapse into unauthorized: delivery must not enumerate subscriptions.
func translateAuth(err error) error {
	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrUnauthorized):
		return ErrUnauthorized
	default:
		return err
	}
}

// nodes returns the cached full node set, building it under singleflight on a
// miss.
func (m *Manager) nodes(sub *storage.Subscription) ([]*model.ProxyNode, error) {
	id := sub.ID
	fp := fingerprint(sub)
	m.cacheMu.RLock()
	item, ok := m.cache[id]
	m.cacheMu.RUnlock()
	if ok && item.def == fp && time.Since(item.fetchedAt) < m.ttl {
		return item.nodes, nil
	}

	call, leader := m.join(id)
	if !leader {
		<-call.done
		return call.nodes, call.err
	}
	// Unconditional handoff: build() does real work (regex, JSON, network
	// results) and a panic there must release the followers waiting on
	// call.done instead of hanging them and stranding the flights entry.
	settled := false
	defer func() {
		if !settled {
			call.err = fmt.Errorf("subscription %q build aborted", id)
			close(call.done)
			m.leave(id, call)
		}
	}()
	nodes, err := m.build(sub, fp)
	call.nodes, call.err = nodes, err
	close(call.done)
	m.leave(id, call)
	settled = true
	return nodes, err
}

// join registers the caller as the builder for id, or attaches it to the
// builder already running.
func (m *Manager) join(id string) (*fetchCall, bool) {
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	if call, ok := m.flights[id]; ok {
		return call, false
	}
	call := &fetchCall{done: make(chan struct{})}
	m.flights[id] = call
	return call, true
}

func (m *Manager) leave(id string, call *fetchCall) {
	m.flightMu.Lock()
	defer m.flightMu.Unlock()
	if m.flights[id] == call {
		delete(m.flights, id)
	}
}

// Invalidate drops cached delivery results for one subscription, so a deleted
// subscription's nodes - which carry credentials - are not retained until the
// TTL lapses.
func (m *Manager) Invalidate(id string) {
	m.cacheMu.Lock()
	delete(m.cache, id)
	m.cacheMu.Unlock()
}

// build fetches every source, applies the pipeline, and republishes the cache.
// Account metadata (expiry/traffic) is gathered per source and merged; it is
// independent of the node pipeline, so filtering nodes never hides the usage.
func (m *Manager) build(sub *storage.Subscription, fp string) ([]*model.ProxyNode, error) {
	var collected []*model.ProxyNode
	var infos []*model.SubscriptionInfo
	var sources []*model.SourceInfo
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, src := range sub.Sources {
		nodes, info, err := m.fetchSource(ctx, src)
		if err != nil {
			continue
		}
		if len(nodes) > 0 {
			collected = append(collected, nodes...)
		}
		if info != nil {
			infos = append(infos, info)
			si := model.SourceInfo{Source: src, SubscriptionInfo: *info}
			sources = append(sources, &si)
		}
	}

	merged := model.MergeSubscriptionInfos(infos)
	processed := m.applyPipeline(collected, sub.Pipeline)

	// Replace this entry and drop anything no longer reachable or usable.
	m.cacheMu.Lock()
	for key, it := range m.cache {
		if time.Since(it.fetchedAt) > m.ttl {
			delete(m.cache, key)
		}
	}
	// A delete landing during this fetch must not be undone by the write:
	// Invalidate()'s promise is that a removed subscription's nodes - with
	// their credentials - are released promptly.
	if _, err := m.store.Get(sub.ID); err != nil {
		delete(m.cache, sub.ID)
		m.cacheMu.Unlock()
		return processed, nil
	}
	m.cache[sub.ID] = &cacheItem{
		nodes:     processed,
		fetchedAt: time.Now(),
		def:       fp,
		info:      merged,
		sources:   sources,
	}
	m.cacheMu.Unlock()

	return processed, nil
}

// UserInfo returns the merged account metadata and the per-source breakdown
// for a subscription, building it (via the same singleflight as the node set)
// if it is not cached. Both are nil when no upstream reported any.
func (m *Manager) UserInfo(id string) (*model.SubscriptionInfo, []*model.SourceInfo, error) {
	sub, err := m.store.Get(id)
	if err != nil {
		return nil, nil, err
	}
	if _, err := m.nodes(sub); err != nil {
		return nil, nil, err
	}
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	if it, ok := m.cache[id]; ok {
		return it.info, it.sources, nil
	}
	return nil, nil, nil
}

func (m *Manager) fetchSource(ctx context.Context, src string) ([]*model.ProxyNode, *model.SubscriptionInfo, error) {
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		return m.fetcher.FetchSubscription(ctx, src, "")
	default:
		// Inline payload: single URI, base64 blob, Clash YAML or sing-box JSON fragment.
		nodes, err := parser.ParseSubscription([]byte(src))
		if err == nil && len(nodes) > 0 {
			return nodes, nil, nil
		}
		return nil, nil, fmt.Errorf("unrecognized source: %s", src)
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
