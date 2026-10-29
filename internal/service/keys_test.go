package service_test

import (
	"os"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/service"
	"github.com/arisvia/subrefine/internal/storage"
)

// Resolve authenticates any attached global key, and editing a (subscription,
// key) blacklist must NOT force an upstream re-fetch: the shared node cache is
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

	alice := &storage.DeliveryKey{Name: "Alice", Token: "tok-a"}
	bob := &storage.DeliveryKey{Name: "Bob", Token: "tok-b"}
	for _, k := range []*storage.DeliveryKey{alice, bob} {
		if err := store.SaveKey(k); err != nil {
			t.Fatalf("save key %s: %v", k.Name, err)
		}
	}

	one := "ss://YWVzLTEyOC1nY206cDpwQDEuMS4xLjE6ODM4OCAjT25l"
	sub := &storage.Subscription{
		ID: "multi", Name: "Multi", Enabled: true, Sources: []string{one},
		Keys: []storage.SubKeyRef{{ID: alice.ID}, {ID: bob.ID}},
	}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	mgr := service.NewManager(store, fetcher.New(time.Second), time.Hour)

	got, ref, nodes, err := mgr.Resolve("multi", "tok-b")
	if err != nil {
		t.Fatalf("resolve secondary key: %v", err)
	}
	if got.ID != "multi" || len(nodes) != 1 {
		t.Fatalf("unexpected resolve result: sub=%v nodes=%d", got.ID, len(nodes))
	}
	if k, err := store.GetKey(ref.ID); err != nil || k.Name != "Bob" {
		t.Fatalf("reference must resolve to Bob's key: %+v %v", ref, err)
	}
	if _, _, _, err := mgr.Resolve("multi", "wrong"); err != service.ErrUnauthorized {
		t.Fatalf("wrong token must be unauthorized, got %v", err)
	}
	// The legacy single-token call path authenticates any attached key too.
	if _, err := mgr.GetSubscription("multi", "tok-a"); err != nil {
		t.Fatalf("GetSubscription must accept a non-primary key: %v", err)
	}

	// Edit Bob's blacklist only. Sources/pipeline are untouched, so the node
	// set must come back from the cache as the very same backing slice.
	sub.Keys[1].Disabled = []string{"deadbeef0000"}
	if err := store.Save(sub); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	_, ref2, nodes2, err := mgr.Resolve("multi", "tok-b")
	if err != nil {
		t.Fatalf("resolve after edit: %v", err)
	}
	if len(ref2.Disabled) != 1 {
		t.Fatalf("blacklist not visible on resolved reference: %+v", ref2)
	}
	if &nodes2[0] != &nodes[0] {
		t.Fatal("key edit triggered an upstream refetch")
	}
}

// One global key attached to two subscriptions authenticates both - the whole
// point of the shared pool - and each pair keeps its own blacklist.
func TestSharedKeyAcrossSubscriptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "subrefine_shared_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	bob := &storage.DeliveryKey{Name: "Bob", Token: "bobtok"}
	if err := store.SaveKey(bob); err != nil {
		t.Fatalf("save key: %v", err)
	}
	mk := func(id string, disabled []string) {
		sub := &storage.Subscription{ID: id, Enabled: true,
			Sources: []string{"ss://YWVzLTEyOC1nY206cDpwQDEuMS4xLjE6ODM4OCAjT25l"},
			Keys:    []storage.SubKeyRef{{ID: bob.ID, Disabled: disabled}}}
		if err := store.Save(sub); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	mk("feed1", []string{"xyz"})
	mk("feed2", nil)

	mgr := service.NewManager(store, fetcher.New(time.Second), time.Hour)
	_, ref1, _, err := mgr.Resolve("feed1", "bobtok")
	if err != nil || len(ref1.Disabled) != 1 {
		t.Fatalf("feed1 pair blacklist wrong: %v %+v", err, ref1)
	}
	_, ref2, _, err := mgr.Resolve("feed2", "bobtok")
	if err != nil || len(ref2.Disabled) != 0 {
		t.Fatalf("feed2 pair must carry no blacklist: %v %+v", err, ref2)
	}
}
