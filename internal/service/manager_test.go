package service_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/service"
	"github.com/arisvia/subrefine/internal/storage"
)

func TestStorageAndService(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "subrefine_test_*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("init storage failed: %v", err)
	}

	sub := &storage.Subscription{
		ID:      "sub-hk",
		Name:    "Hong Kong Cluster",
		Token:   "secret-token",
		Enabled: true,
		Sources: []string{
			"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@1.1.1.1:8388#HK-01",
			"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@1.1.1.1:8388#HK-01-Duplicate",
			"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@2.2.2.2:8388#US-01",
		},
		Pipeline: []storage.PipelineStep{
			{
				Type:    "filter",
				Include: "HK",
			},
			{
				Type:    "rename",
				Pattern: "HK-(\\d+)",
				Repl:    "🇭🇰 Hong Kong $1",
			},
			{
				Type: "deduplicate",
			},
			{
				Type:        "decorate",
				Fingerprint: "safari",
				Detour:      "WARP",
			},
		},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	if err := store.Save(sub); err != nil {
		t.Fatalf("save sub failed: %v", err)
	}

	// Verify persistence file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "subscriptions.json")); err != nil {
		t.Fatalf("persistence file not created: %v", err)
	}

	f := fetcher.New(5 * time.Second)
	mgr := service.NewManager(store, f, 1*time.Minute)

	// Unauthorized test
	_, err = mgr.GetSubscription("sub-hk", "wrong-token")
	if err == nil {
		t.Fatalf("expected unauthorized error")
	}

	// Authorized test
	nodes, err := mgr.GetSubscription("sub-hk", "secret-token")
	if err != nil {
		t.Fatalf("get sub failed: %v", err)
	}

	// Filter retained HK (US dropped), deduplicate removed HK duplicate -> exactly 1 node
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	node := nodes[0]
	if node.Tag != "🇭🇰 Hong Kong 01" {
		t.Fatalf("unexpected tag after rename: %s", node.Tag)
	}
	if node.Fingerprint != "safari" || node.Detour != "WARP" {
		t.Fatalf("unexpected decoration values: fp=%s detour=%s", node.Fingerprint, node.Detour)
	}
}

// Inline sources must survive: a base64 subscription blob pasted straight into
// the source list, a bare protocol URI, and sources whose length happens to be
// exactly 7 bytes (which used to index out of range while sniffing "http://").
func TestInlineSourcesAreDetected(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "subrefine_inline_*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("init storage failed: %v", err)
	}

	blob := base64.StdEncoding.EncodeToString([]byte(
		"ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@1.1.1.1:8388#Inline-A\n" +
			"trojan://tp@2.2.2.2:443#Inline-B\n"))

	sub := &storage.Subscription{
		ID:      "inline",
		Token:   "tok",
		Enabled: true,
		Sources: []string{
			blob,
			"vless://aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee@3.3.3.3:443",
			"1234567",
		},
	}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save sub failed: %v", err)
	}

	mgr := service.NewManager(store, fetcher.New(time.Second), time.Minute)
	nodes, err := mgr.GetSubscription("inline", "tok")
	if err != nil {
		t.Fatalf("get sub failed: %v", err)
	}

	tags := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		tags[n.Tag] = true
	}
	for _, want := range []string{"Inline-A", "Inline-B", "VLESS-3.3.3.3:443"} {
		if !tags[want] {
			t.Fatalf("source payload lost %q, got tags %v", want, tags)
		}
	}
}

// Editing a subscription must take effect on the very next request: the node
// cache is pinned to the subscription definition, not just to its id + TTL.
func TestEditTakesEffectImmediately(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "subrefine_edit_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("init storage: %v", err)
	}

	one := "ss://YWVzLTEyOC1nY206cDpwQDEuMS4xLjE6ODM4OCAjT25l"
	two := "trojan://tp@2.2.2.2:443#Two"

	if err := store.Save(&storage.Subscription{ID: "live", Token: "t", Enabled: true, Sources: []string{one}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	mgr := service.NewManager(store, fetcher.New(time.Second), time.Hour)
	nodes, err := mgr.GetSubscription("live", "t")
	if err != nil || len(nodes) != 1 {
		t.Fatalf("first read: %d nodes, err %v", len(nodes), err)
	}

	// Same id, changed definition: sources plus a protocol filter.
	if err := store.Save(&storage.Subscription{
		ID: "live", Token: "t", Enabled: true, Sources: []string{one, two},
		Pipeline: []storage.PipelineStep{{Type: "filter", Types: []string{model.ProtoTrojan}}},
	}); err != nil {
		t.Fatalf("re-save: %v", err)
	}

	nodes, err = mgr.GetSubscription("live", "t")
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != model.ProtoTrojan {
		t.Fatalf("stale cache served after edit: %+v", nodes)
	}

	// Unchanged definition still hits the cache: same backing array, no refetch.
	cached, err := mgr.GetSubscription("live", "t")
	if err != nil || len(cached) != 1 {
		t.Fatalf("cached read: %d nodes, err %v", len(cached), err)
	}
	if &cached[0] != &nodes[0] {
		t.Fatal("expected cache hit to return the previously computed slice")
	}
}
