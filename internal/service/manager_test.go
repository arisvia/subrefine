package service_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
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
