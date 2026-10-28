package storage_test

import (
	"os"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/storage"
)

func TestSaveStampsTimestamps(t *testing.T) {
	dir, err := os.MkdirTemp("", "subrefine_store_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	sub := &storage.Subscription{ID: "a", Token: "t", Enabled: true, Sources: []string{"x"}}
	if err := store.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Get("a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CreatedAt == 0 || got.UpdatedAt == 0 {
		t.Fatalf("timestamps not stamped: %+v", got)
	}
	first := got.CreatedAt

	// A later edit must keep the original creation time and move UpdatedAt.
	time.Sleep(1100 * time.Millisecond)
	edit := &storage.Subscription{ID: "a", Token: "t2", Enabled: true, Sources: []string{"x", "y"}}
	if err := store.Save(edit); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	got, _ = store.Get("a")
	if got.CreatedAt != first {
		t.Fatalf("created_at rewritten on update: %d -> %d", first, got.CreatedAt)
	}
	if got.UpdatedAt <= first {
		t.Fatalf("updated_at not advanced: %d", got.UpdatedAt)
	}
	if got.Token != "t2" || len(got.Sources) != 2 {
		t.Fatalf("update lost fields: %+v", got)
	}

	// Reload from disk: the atomic file must carry the same rows.
	reopened, err := storage.New(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list := reopened.List()
	if len(list) != 1 || list[0].ID != "a" || list[0].CreatedAt != first {
		t.Fatalf("reload mismatch: %+v", list)
	}
}
