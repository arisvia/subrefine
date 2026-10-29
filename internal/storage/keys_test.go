package storage_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/arisvia/subrefine/internal/storage"
)

func writeData(t *testing.T, dir, name string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0644); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
}

// A data file written before multi-key support (no "keys" array) must load
// into the global model with the legacy token as an attached key, so every
// link already in the wild keeps working.
func TestLoadMigratesLegacyToken(t *testing.T) {
	dir := t.TempDir()
	writeData(t, dir, "subscriptions.json", []map[string]any{{
		"id": "old", "token": "legacytok",
		"sources": []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"}, "enabled": true,
	}})

	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	sub, err := store.Get("old")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(sub.Keys) != 1 {
		t.Fatalf("legacy token not migrated into a key reference: %+v", sub.Keys)
	}
	if sub.Token != "legacytok" {
		t.Fatalf("primary token out of sync: %q", sub.Token)
	}
	got, key, ref, err := store.Authenticate("old", "legacytok")
	if err != nil || got.ID != "old" || key == nil || ref == nil {
		t.Fatalf("Authenticate must accept the migrated token: %v", err)
	}
}

// The pre-global model embedded keys inside each subscription. Two
// subscriptions carrying the SAME token must converge on ONE global key (same
// secret = same credential); distinct tokens become distinct keys.
func TestLoadPromotesEmbeddedKeys(t *testing.T) {
	dir := t.TempDir()
	writeData(t, dir, "subscriptions.json", []map[string]any{
		{"id": "a", "token": "tokA", "enabled": true,
			"keys": []map[string]any{{"id": "default", "name": "Bob", "token": "tokA"}}},
		{"id": "b", "token": "tokA", "enabled": true,
			"keys": []map[string]any{{"id": "default", "name": "Bob", "token": "tokA"},
				{"id": "k2", "name": "Phone", "token": "tokB", "disabled": []string{"abc123def456"}}}},
	})

	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	keys := store.ListKeys()
	if len(keys) != 2 {
		t.Fatalf("shared token must converge to one key, got %d keys: %+v", len(keys), keys)
	}
	subA, _ := store.Get("a")
	subB, _ := store.Get("b")
	if subA.Keys[0].ID != subB.Keys[0].ID {
		t.Fatal("both subscriptions must reference the same global key for tokA")
	}
	// tokB belongs only to b; authenticating it against a must fail.
	if _, _, _, err := store.Authenticate("a", "tokB"); !errors.Is(err, storage.ErrUnauthorized) {
		t.Fatalf("unattached valid token must be unauthorized, got %v", err)
	}
	_, _, ref, err := store.Authenticate("b", "tokB")
	if err != nil {
		t.Fatalf("attached token must authenticate: %v", err)
	}
	if len(ref.Disabled) != 1 || ref.Disabled[0] != "abc123def456" {
		t.Fatalf("blacklist must survive promotion: %+v", ref.Disabled)
	}

	// Migration is one-shot: reopening must not rewrite ids or tokens.
	again, err := storage.New(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	subA2, _ := again.Get("a")
	if subA2.Keys[0].ID != subA.Keys[0].ID || subA2.Token != "tokA" {
		t.Fatal("migrated state must be stable across restarts")
	}
}

func TestAuthenticateRejects(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	key := &storage.DeliveryKey{Name: "Bob", Token: "onetwo"}
	if err := store.SaveKey(key); err != nil {
		t.Fatalf("save key: %v", err)
	}
	sub := &storage.Subscription{ID: "s1", Name: "S1", Enabled: true,
		Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"},
		Keys:    []storage.SubKeyRef{{ID: key.ID}}}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save sub: %v", err)
	}

	if _, _, _, err := store.Authenticate("s1", ""); !errors.Is(err, storage.ErrUnauthorized) {
		t.Fatalf("empty token: want unauthorized, got %v", err)
	}
	if _, _, _, err := store.Authenticate("s1", "onetxw"); !errors.Is(err, storage.ErrUnauthorized) {
		t.Fatalf("wrong token: want unauthorized, got %v", err)
	}
	if _, _, _, err := store.Authenticate("nosuch", "onetwo"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("unknown sub: want not found, got %v", err)
	}
	// A second subscription without the key must reject the same token.
	other := &storage.Subscription{ID: "s2", Name: "S2", Enabled: true,
		Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"}}
	if err := store.Save(other); err != nil {
		t.Fatalf("save other: %v", err)
	}
	if _, _, _, err := store.Authenticate("s2", "onetwo"); !errors.Is(err, storage.ErrUnauthorized) {
		t.Fatalf("token not attached to s2 must be unauthorized, got %v", err)
	}
}

// A subscription saved without references gets a fresh default key so it is
// always deliverable; an unknown reference id is rejected outright.
func TestSaveAutoDefaultAndUnknownRef(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	sub := &storage.Subscription{ID: "s1", Enabled: true,
		Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"}}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(sub.Keys) != 1 || sub.Token == "" {
		t.Fatalf("default key not auto-created: %+v token=%q", sub.Keys, sub.Token)
	}
	if _, _, _, err := store.Authenticate("s1", sub.Token); err != nil {
		t.Fatalf("auto default must authenticate: %v", err)
	}

	bad := &storage.Subscription{ID: "s2", Enabled: true,
		Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"},
		Keys:    []storage.SubKeyRef{{ID: "ghost"}}}
	if err := store.Save(bad); !errors.Is(err, storage.ErrUnknownKey) {
		t.Fatalf("unknown key ref must be rejected, got %v", err)
	}
}

func TestSaveKeyTokenUniquenessAndRotation(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	a := &storage.DeliveryKey{Name: "A", Token: "shared"}
	if err := store.SaveKey(a); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := store.SaveKey(&storage.DeliveryKey{Name: "B", Token: "shared"}); !errors.Is(err, storage.ErrTokenTaken) {
		t.Fatalf("duplicate token must be rejected, got %v", err)
	}

	sub := &storage.Subscription{ID: "s1", Enabled: true,
		Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"},
		Keys:    []storage.SubKeyRef{{ID: a.ID}}}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save sub: %v", err)
	}
	rotated := *a
	rotated.Token = "fresh"
	if err := store.SaveKey(&rotated); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	after, _ := store.Get("s1")
	if after.Token != "fresh" {
		t.Fatalf("rotation must re-sync mirrored token, got %q", after.Token)
	}
	if _, _, _, err := store.Authenticate("s1", "fresh"); err != nil {
		t.Fatalf("new token must authenticate: %v", err)
	}
	if _, _, _, err := store.Authenticate("s1", "shared"); !errors.Is(err, storage.ErrUnauthorized) {
		t.Fatalf("old token must be dead after rotation, got %v", err)
	}
}

func TestDeleteKeyUsageAndCascade(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	k := &storage.DeliveryKey{Name: "Bob", Token: "tok"}
	if err := store.SaveKey(k); err != nil {
		t.Fatalf("save key: %v", err)
	}
	if got := store.KeyUsage(k.ID); got == nil || len(got) != 0 {
		t.Fatalf("unused key must report [] (never null on the wire): %#v", got)
	}
	for _, id := range []string{"s1", "s2"} {
		sub := &storage.Subscription{ID: id, Enabled: true,
			Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"},
			Keys:    []storage.SubKeyRef{{ID: k.ID}}}
		if err := store.Save(sub); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	usage := store.KeyUsage(k.ID)
	if len(usage) != 2 || usage[0] != "s1" || usage[1] != "s2" {
		t.Fatalf("usage list wrong: %+v", usage)
	}
	if err := store.DeleteKey(k.ID, false); !errors.Is(err, storage.ErrInUse) {
		t.Fatalf("in-use delete must be blocked, got %v", err)
	}
	if err := store.DeleteKey(k.ID, true); err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	// Both subs were left keyless and got fresh defaults with new tokens.
	s1, _ := store.Get("s1")
	s2, _ := store.Get("s2")
	if s1.Token == "tok" || s2.Token == "tok" || s1.Token == "" || s2.Token == "" {
		t.Fatalf("cascade must issue fresh default keys: s1=%q s2=%q", s1.Token, s2.Token)
	}
	if _, _, _, err := store.Authenticate("s1", s1.Token); err != nil {
		t.Fatalf("fresh default must authenticate: %v", err)
	}
}
