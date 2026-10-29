package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/arisvia/subrefine/internal/storage"
)

// A data file written before multi-key support (no "keys" array) must load
// into the new model with the legacy token as the primary key, so every link
// already in the wild keeps working.
func TestNormalizeMigratesLegacyToken(t *testing.T) {
	dir := t.TempDir()
	legacy := []struct {
		ID      string   `json:"id"`
		Token   string   `json:"token"`
		Sources []string `json:"sources"`
		Enabled bool     `json:"enabled"`
	}{{ID: "old", Token: "legacytok", Sources: []string{"ss://YWVzLTEyOC1nY206cHA=@1.1.1.1:8388#A"}, Enabled: true}}
	raw, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.json"), raw, 0644); err != nil {
		t.Fatalf("seed legacy file: %v", err)
	}

	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	sub, err := store.Get("old")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(sub.Keys) != 1 || sub.Keys[0].Token != "legacytok" {
		t.Fatalf("legacy token not migrated into keys: %+v", sub.Keys)
	}
	if sub.Token != "legacytok" {
		t.Fatalf("primary token out of sync: %q", sub.Token)
	}
	if sub.FindKey("legacytok") == nil {
		t.Fatal("FindKey must accept the migrated token")
	}
}

func TestNormalizeGeneratesMissingKeys(t *testing.T) {
	sub := &storage.Subscription{ID: "fresh", Enabled: true}
	if err := sub.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(sub.Keys) != 1 || len(sub.Keys[0].Token) != 32 {
		t.Fatalf("expected one generated 32-hex token, got %+v", sub.Keys)
	}
	first := sub.Keys[0].Token

	// Idempotent: a second pass must not rotate the generated token.
	if err := sub.Normalize(); err != nil {
		t.Fatalf("re-normalize: %v", err)
	}
	if sub.Keys[0].Token != first {
		t.Fatal("Normalize rotated an existing token")
	}
}

func TestFindKeyMatchesAnyKey(t *testing.T) {
	sub := &storage.Subscription{
		ID: "multi",
		Keys: []storage.SubKey{
			{ID: "a", Name: "A", Token: "tok-a"},
			{ID: "b", Name: "B", Token: "tok-b", Disabled: []string{"deadbeef0000"}},
		},
	}
	if k := sub.FindKey("tok-b"); k == nil || k.ID != "b" || len(k.Disabled) != 1 {
		t.Fatalf("secondary key lookup failed: %+v", k)
	}
	if k := sub.FindKey("tok-a"); k == nil || k.ID != "a" {
		t.Fatal("primary key lookup failed")
	}
	if sub.FindKey("nope") != nil || sub.FindKey("") != nil {
		t.Fatal("unknown/empty token must not match a key")
	}
}

func TestSaveRejectsDuplicateTokens(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	sub := &storage.Subscription{
		ID: "dup", Enabled: true,
		Keys: []storage.SubKey{
			{ID: "a", Name: "A", Token: "same"},
			{ID: "b", Name: "B", Token: "same"},
		},
	}
	if err := store.Save(sub); err == nil {
		t.Fatal("Save must reject two keys sharing a token")
	}
	if got := (&storage.Subscription{Keys: sub.Keys}).DuplicateTokenID(); got != "a" {
		t.Fatalf("DuplicateTokenID should report the first holder, got %q", got)
	}
}
