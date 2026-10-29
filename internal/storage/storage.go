package storage

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("subscription not found")
	ErrUnauthorized = errors.New("unauthorized delivery token")
	// ErrTokenTaken rejects a second key carrying a token that already
	// authenticates: one token must map to exactly one identity.
	ErrTokenTaken = errors.New("delivery token already in use")
	// ErrInUse blocks deleting a key that subscriptions still attach; the
	// caller decides whether to force a cascade detach.
	ErrInUse      = errors.New("delivery key is attached to subscriptions")
	ErrUnknownKey = errors.New("unknown delivery key")
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

// SubKeyRef attaches a global delivery key to a subscription. The blacklist of
// withheld node identities belongs to the (subscription, key) pair, not to the
// key: identities are content hashes of this subscription's node set, so the
// same key attached elsewhere carries no blacklist there.
type SubKeyRef struct {
	ID       string   `json:"id"`
	Disabled []string `json:"disabled,omitempty"`
}

// DeliveryKey is a first-class credential shared by any number of
// subscriptions. One person or device gets one key; every subscription that
// attaches it serves that link, so rotation and revocation happen in one place.
type DeliveryKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type Subscription struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Token mirrors the first attached key's token, kept in sync by the store.
	// Delivery authenticates against every attached key, not just this one.
	Token    string         `json:"token"`
	Keys     []SubKeyRef    `json:"keys,omitempty"`
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

// legacyKey is the pre-global wire shape: keys embedded in the subscription
// with their token inline. A non-empty Token on load means "promote me".
type legacyKey struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

func generateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func generateKeyID() (string, error) {
	t, err := generateToken()
	if err != nil {
		return "", err
	}
	return "k" + t[:8], nil
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	keysPath string
	subs     map[string]*Subscription
	keys     map[string]*DeliveryKey
}

func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir failed: %w", err)
	}

	store := &Store{
		filePath: filepath.Join(dataDir, "subscriptions.json"),
		keysPath: filepath.Join(dataDir, "keys.json"),
		subs:     make(map[string]*Subscription),
		keys:     make(map[string]*DeliveryKey),
	}

	if err := store.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load storage failed: %w", err)
	}

	return store, nil
}

// load reads keys first (subscriptions reference them), then subscriptions,
// promoting any legacy embedded keys into the global pool and healing
// dangling references. If anything changed on disk shape, both files are
// rewritten so the migration runs exactly once.
func (s *Store) load() error {
	healed, err := s.loadKeys()
	if err != nil {
		return err
	}
	changed := healed

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		var list []*Subscription
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		// Best-effort second decode that keeps the legacy embedded tokens;
		// the primary decode above silently drops them.
		var wire []struct {
			Keys []legacyKey `json:"keys"`
		}
		_ = json.Unmarshal(data, &wire)

		for i, sub := range list {
			if i < len(wire) && s.promoteLegacy(sub, wire[i].Keys) {
				changed = true
			}
			created, err := s.ensureRefs(sub)
			if err != nil {
				return err
			}
			if created {
				changed = true
			}
			s.subs[sub.ID] = sub
		}
	}

	if changed {
		if err := s.save(); err != nil {
			return err
		}
		return s.saveKeys()
	}
	return nil
}

// loadKeys reads keys.json when present. Duplicate tokens (possible only via
// hand-edited data) are healed by regenerating the later key's token.
func (s *Store) loadKeys() (bool, error) {
	data, err := os.ReadFile(s.keysPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	var list []*DeliveryKey
	if err := json.Unmarshal(data, &list); err != nil {
		return false, err
	}
	changed := false
	seen := make(map[string]bool, len(list))
	for _, k := range list {
		if k.ID == "" {
			id, err := generateKeyID()
			if err != nil {
				return false, err
			}
			k.ID = id
			changed = true
		}
		if k.Token == "" {
			t, err := generateToken()
			if err != nil {
				return false, err
			}
			k.Token = t
			changed = true
		}
		if seen[k.Token] {
			t, err := generateToken()
			if err != nil {
				return false, err
			}
			k.Token = t
			changed = true
		}
		seen[k.Token] = true
		if k.Name == "" {
			k.Name = "Key"
			changed = true
		}
		s.keys[k.ID] = k
	}
	return changed, nil
}

// promoteLegacy moves embedded key tokens into the global pool. Two
// subscriptions that carried the same token converge on one global key: the
// same secret IS the same credential, and that is the sharing the model is
// for. Returns true when the subscription was rewritten.
func (s *Store) promoteLegacy(sub *Subscription, legacy []legacyKey) bool {
	changed := false
	for i := range sub.Keys {
		if i >= len(legacy) || legacy[i].Token == "" {
			continue // already a reference
		}
		if existing := s.keyByToken(legacy[i].Token); existing != nil {
			sub.Keys[i].ID = existing.ID
			changed = true
			continue
		}
		id := sub.Keys[i].ID
		if id == "" || s.keys[id] != nil {
			fresh, err := generateKeyID()
			if err != nil {
				fresh = "k" + sub.ID
			}
			id = fresh
		}
		name := legacy[i].Name
		if name == "" {
			name = "Key"
		}
		now := time.Now().Unix()
		s.keys[id] = &DeliveryKey{ID: id, Name: name, Token: legacy[i].Token, CreatedAt: now, UpdatedAt: now}
		sub.Keys[i].ID = id
		changed = true
	}
	return changed
}

// ensureRefs sanitizes a subscription's key references: unknown, empty and
// duplicate ids are dropped, an empty list gets a default key created (or an
// existing key with the legacy primary token reused), and Token is re-synced
// from the first reference. Callers hold the lock (load runs before serving).
func (s *Store) ensureRefs(sub *Subscription) (bool, error) {
	seen := make(map[string]bool, len(sub.Keys))
	refs := make([]SubKeyRef, 0, len(sub.Keys))
	for _, r := range sub.Keys {
		if r.ID == "" || s.keys[r.ID] == nil || seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		refs = append(refs, r)
	}
	sub.Keys = refs

	created := false
	if len(sub.Keys) == 0 {
		token := sub.Token
		if token == "" {
			t, err := generateToken()
			if err != nil {
				return false, err
			}
			token = t
		}
		if dup := s.keyByToken(token); dup != nil {
			sub.Keys = []SubKeyRef{{ID: dup.ID}}
		} else {
			id, err := generateKeyID()
			if err != nil {
				return false, err
			}
			now := time.Now().Unix()
			s.keys[id] = &DeliveryKey{ID: id, Name: "Default", Token: token, CreatedAt: now, UpdatedAt: now}
			sub.Keys = []SubKeyRef{{ID: id}}
			created = true
		}
	}
	sub.Token = s.keys[sub.Keys[0].ID].Token
	return created, nil
}

// keyByToken finds the global key carrying this exact token. Callers hold the
// lock; it is used for admin-side uniqueness and migration, never for
// delivery, which goes through Authenticate.
func (s *Store) keyByToken(token string) *DeliveryKey {
	if token == "" {
		return nil
	}
	for _, k := range s.keys {
		if k.Token == token {
			return k
		}
	}
	return nil
}

func (s *Store) save() error {
	list := s.listLocked()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.filePath, data)
}

func (s *Store) saveKeys() error {
	list := s.listKeysLocked()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.keysPath, data)
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
	return s.listLocked()
}

// listLocked returns subscriptions in creation order; callers hold the lock.
func (s *Store) listLocked() []*Subscription {
	list := make([]*Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		list = append(list, sub)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt != list[j].CreatedAt {
			return list[i].CreatedAt < list[j].CreatedAt
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// Save stores a subscription. Every referenced key must exist (a stale id is
// a console bug or a hand-edit, never a silent drop); a subscription with no
// references at all gets a fresh default key so it is always deliverable.
func (s *Store) Save(sub *Subscription) error {
	if err := ValidateID(sub.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range sub.Keys {
		if r.ID == "" {
			return errors.New("delivery key reference requires an id")
		}
		if s.keys[r.ID] == nil {
			return fmt.Errorf("%w %q", ErrUnknownKey, r.ID)
		}
	}

	now := time.Now().Unix()
	if sub.CreatedAt == 0 {
		if old, ok := s.subs[sub.ID]; ok && old.CreatedAt != 0 {
			sub.CreatedAt = old.CreatedAt
		} else {
			sub.CreatedAt = now
		}
	}
	sub.UpdatedAt = now

	created, err := s.ensureRefs(sub)
	if err != nil {
		return err
	}
	s.subs[sub.ID] = sub
	if err := s.save(); err != nil {
		return err
	}
	if created {
		return s.saveKeys()
	}
	return nil
}

// Authenticate resolves a delivery token against the global key pool and
// checks that the subscription attaches it. The constant-time scan over every
// key keeps the attach check from becoming a token oracle: a valid token for
// another subscription and a wholly invalid token answer identically slow.
func (s *Store) Authenticate(subID, token string) (*Subscription, *DeliveryKey, *SubKeyRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subs[subID]
	if !ok {
		return nil, nil, nil, ErrNotFound
	}
	if token == "" {
		return nil, nil, nil, ErrUnauthorized
	}
	var match *DeliveryKey
	for _, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(k.Token)) == 1 {
			match = k
		}
	}
	if match == nil {
		return nil, nil, nil, ErrUnauthorized
	}
	for i := range sub.Keys {
		if sub.Keys[i].ID == match.ID {
			return sub, match, &sub.Keys[i], nil
		}
	}
	return nil, nil, nil, ErrUnauthorized
}

func (s *Store) ListKeys() []*DeliveryKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listKeysLocked()
}

// listKeysLocked returns keys in creation order; callers hold the lock.
func (s *Store) listKeysLocked() []*DeliveryKey {
	list := make([]*DeliveryKey, 0, len(s.keys))
	for _, k := range s.keys {
		list = append(list, k)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt != list[j].CreatedAt {
			return list[i].CreatedAt < list[j].CreatedAt
		}
		return list[i].ID < list[j].ID
	})
	return list
}

func (s *Store) GetKey(id string) (*DeliveryKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return nil, ErrNotFound
	}
	return k, nil
}

// KeyUsage lists subscription ids attaching this key, sorted for stable UI.
// Never nil: an unused key answers [] so console code can trust in_use.
func (s *Store) KeyUsage(id string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	used := []string{}
	for sid, sub := range s.subs {
		for _, r := range sub.Keys {
			if r.ID == id {
				used = append(used, sid)
				break
			}
		}
	}
	sort.Strings(used)
	return used
}

// SaveKey upserts a global key. Missing id/token are generated; a token that
// another key already carries is rejected, because delivery resolves tokens
// to exactly one identity. Rotating the token of an attached key changes
// every link that uses it - the console says so.
func (s *Store) SaveKey(k *DeliveryKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if k.ID == "" {
		id, err := generateKeyID()
		if err != nil {
			return err
		}
		k.ID = id
	}
	if !idPattern.MatchString(k.ID) {
		return fmt.Errorf("delivery key id %q must match [A-Za-z0-9_-]{1,64}", k.ID)
	}
	if k.Name == "" {
		k.Name = "Key"
	}
	if k.Token == "" {
		t, err := generateToken()
		if err != nil {
			return err
		}
		k.Token = t
	}
	if other := s.keyByToken(k.Token); other != nil && other.ID != k.ID {
		return ErrTokenTaken
	}

	now := time.Now().Unix()
	rotated := false
	if old, ok := s.keys[k.ID]; ok {
		k.CreatedAt = old.CreatedAt
		rotated = old.Token != k.Token
	} else {
		k.CreatedAt = now
	}
	k.UpdatedAt = now
	s.keys[k.ID] = k

	if err := s.saveKeys(); err != nil {
		return err
	}
	if rotated {
		// Re-sync the mirrored primary token of every attaching subscription.
		for _, sub := range s.subs {
			if len(sub.Keys) > 0 && sub.Keys[0].ID == k.ID {
				sub.Token = k.Token
			}
		}
		return s.save()
	}
	return nil
}

// DeleteKey removes a global key. While subscriptions attach it, deletion
// fails with ErrInUse unless cascade is set; a cascade detaches the key
// everywhere, and any subscription left with no keys gets a fresh default
// (its links change - the console confirms this).
func (s *Store) DeleteKey(id string, cascade bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keys[id] == nil {
		return ErrNotFound
	}
	var affected []*Subscription
	for _, sub := range s.subs {
		for _, r := range sub.Keys {
			if r.ID == id {
				affected = append(affected, sub)
				break
			}
		}
	}
	if len(affected) > 0 && !cascade {
		return ErrInUse
	}
	// Drop the key BEFORE healing the subscriptions: ensureRefs mirrors an
	// empty subscription's primary token into a new key, and a revoked token
	// must never resurrect - it gets a fresh one instead.
	delete(s.keys, id)
	for _, sub := range affected {
		refs := make([]SubKeyRef, 0, len(sub.Keys))
		for _, r := range sub.Keys {
			if r.ID != id {
				refs = append(refs, r)
			}
		}
		sub.Keys = refs
		if len(refs) == 0 {
			sub.Token = ""
		}
		if _, err := s.ensureRefs(sub); err != nil {
			return err
		}
	}

	if err := s.save(); err != nil {
		return err
	}
	return s.saveKeys()
}

// ValidateID keeps an id usable in a URL path segment and inside the data
// file. It mirrors the console's ID_PATTERN (webui SubscriptionEditor): ids
// may only use unambiguous ASCII. Store.load bypasses Save, so pre-existing
// non-matching ids in an old data file still open; new or edited ones must
// conform.
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
