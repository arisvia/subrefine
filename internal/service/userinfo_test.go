package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/service"
	"github.com/arisvia/subrefine/internal/storage"
)

// Two upstream accounts behind one merged subscription: the delivered view
// sums usage and shows the earliest expiry, while the per-source breakdown
// stays available for the console.
func TestUserInfoAggregationAcrossSources(t *testing.T) {
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("subscription-userinfo", "upload=100; download=200; total=1000; expire=900")
		w.Write([]byte("ss://YWVzLTEyOC1nY206cA==@1.1.1.1:8388#A"))
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=3000; expire=500")
		w.Write([]byte("trojan://tp@2.2.2.2:443#B"))
	}))
	defer b.Close()

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save(&storage.Subscription{
		ID: "merged", Token: "t", Enabled: true,
		Sources: []string{a.URL, b.URL},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	mgr := service.NewManager(store, fetcher.New(5*time.Second), time.Hour)
	if _, err := mgr.GetSubscription("merged", "t"); err != nil {
		t.Fatalf("build: %v", err)
	}

	info, sources, err := mgr.UserInfo("merged")
	if err != nil {
		t.Fatalf("userinfo: %v", err)
	}
	if info == nil || info.Upload != 101 || info.Download != 202 || info.Total != 4000 || info.Expire != 500 {
		t.Fatalf("aggregated wrong: %+v", info)
	}
	if len(sources) != 2 || sources[0].Expire != 900 || sources[1].Expire != 500 {
		t.Fatalf("per-source breakdown wrong: %+v", sources)
	}
}
