package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("subscription not found")
)

type PipelineStep struct {
	Type        string   `json:"type"` // rename, filter, deduplicate, decorate, sort
	Pattern     string   `json:"pattern,omitempty"`
	Repl        string   `json:"repl,omitempty"`
	Include     string   `json:"include,omitempty"`
	Exclude     string   `json:"exclude,omitempty"`
	Types       []string `json:"types,omitempty"`       // filter: protocol allow-list
	Fingerprint string   `json:"fingerprint,omitempty"` // decorate
	Detour      string   `json:"detour,omitempty"`      // decorate
	By          string   `json:"by,omitempty"`          // sort: tag | protocol
	Order       string   `json:"order,omitempty"`       // sort: asc | desc
}

type Subscription struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Token    string         `json:"token"`
	Sources  []string       `json:"sources"` // URLs or local nodes
	Pipeline []PipelineStep `json:"pipeline"`
	Enabled  bool           `json:"enabled"`
	// SingboxMode/ClashMode name a full-config template mode (tun, tproxy,
	// mixed, ...) served to that target instead of the plain fragment.
	// Empty means fragment; the console lists loadable modes from the
	// server's template directory.
	SingboxMode string `json:"singbox_mode,omitempty"`
	ClashMode   string `json:"clash_mode,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	subs     map[string]*Subscription
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir failed: %w", err)
	}

	fp := filepath.Join(dataDir, "subscriptions.json")
	store := &Store{
		filePath: fp,
		subs:     make(map[string]*Subscription),
	}

	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load storage failed: %w", err)
	}

	return store, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var list []*Subscription
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	for _, sub := range list {
		s.subs[sub.ID] = sub
	}
	return nil
}

func (s *Store) save() error {
	list := make([]*Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		list = append(list, sub)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) Get(id string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return sub, nil
}

func (s *Store) List() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		list = append(list, sub)
	}
	return list
}

func (s *Store) Save(sub *Subscription) error {
	if err := ValidateID(sub.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if sub.CreatedAt == 0 {
		if old, ok := s.subs[sub.ID]; ok && old.CreatedAt != 0 {
			sub.CreatedAt = old.CreatedAt
		} else {
			sub.CreatedAt = now
		}
	}
	sub.UpdatedAt = now

	s.subs[sub.ID] = sub
	return s.save()
}

// ValidateID keeps an id usable in a URL path segment and inside the data file.
// It mirrors the console's ID_PATTERN (webui SubscriptionEditor): ids may only
// use unambiguous ASCII. Store.load bypasses Save, so pre-existing non-matching
// ids in an old data file still open; new or edited ones must conform.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func ValidateID(id string) error {
	if id == "" {
		return errors.New("subscription id is required")
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("subscription id %q must match [A-Za-z0-9_-]{1,64}", id)
	}
	return nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.subs[id]; !ok {
		return ErrNotFound
	}
	delete(s.subs, id)
	return s.save()
}
