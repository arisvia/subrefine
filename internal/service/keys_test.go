package service_test

import (
	"os"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/service"
	"github.com/arisvia/subrefine/internal/storage"
)

// Resolve authenticates any key of the subscription, and editing a key's
// blacklist must NOT force an upstream re-fetch: the shared node cache is
// pinned to the node-affecting definition (sources + pipeline) only.
func TestResolveAndKeyEditsStayCached(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "subrefine_keys_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	one := "ss://YWVzLTEyOC1nY206cDpwQDEuMS4xLjE6ODM4OCAjT25l"
	sub := &storage.Subscription{
		ID: "multi", Name: "Multi", Enabled: true, Sources: []string{one},
		Keys: []storage.SubKey{
			{ID: "a", Name: "Alice", Token: "tok-a"},
			{ID: "b", Name: "Bob", Token: "tok-b"},
		},
	}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	mgr := service.NewManager(store, fetcher.New(time.Second), time.Hour)

	got, key, nodes, err := mgr.Resolve("multi", "tok-b")
	if err != nil {
		t.Fatalf("resolve secondary key: %v", err)
	}
	if key.Name != "Bob" || got.ID != "multi" || len(nodes) != 1 {
		t.Fatalf("unexpected resolve result: sub=%v key=%+v nodes=%d", got.ID, key, len(nodes))
	}
	if _, _, _, err := mgr.Resolve("multi", "wrong"); err != service.ErrUnauthorized {
		t.Fatalf("wrong token must be unauthorized, got %v", err)
	}
	// The legacy single-token call path authenticates any key too.
	if _, err := mgr.GetSubscription("multi", "tok-a"); err != nil {
		t.Fatalf("GetSubscription must accept a non-primary key: %v", err)
	}

	// Edit Bob's blacklist only. Sources/pipeline are untouched, so the node
	// set must come back from the cache as the very same backing slice.
	sub.Keys[1].Disabled = []string{"deadbeef0000"}
	if err := store.Save(sub); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	_, key2, nodes2, err := mgr.Resolve("multi", "tok-b")
	if err != nil {
		t.Fatalf("resolve after edit: %v", err)
	}
	if len(key2.Disabled) != 1 {
		t.Fatalf("blacklist not visible on resolved key: %+v", key2)
	}
	if &nodes2[0] != &nodes[0] {
		t.Fatal("key edit triggered an upstream refetch")
	}
}
