package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrNotFound = errors.New("subscription not found")
)

type PipelineStep struct {
	Type        string `json:"type"` // rename, filter, deduplicate, decorate
	Pattern     string `json:"pattern,omitempty"`
	Repl        string `json:"repl,omitempty"`
	Include     string `json:"include,omitempty"`
	Exclude     string `json:"exclude,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Detour      string `json:"detour,omitempty"`
}

type Subscription struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Token     string         `json:"token"`
	Sources   []string       `json:"sources"` // URLs or local nodes
	Pipeline  []PipelineStep `json:"pipeline"`
	Enabled   bool           `json:"enabled"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
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
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[sub.ID] = sub
	return s.save()
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
