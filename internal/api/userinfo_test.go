package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/storage"
)

// Deliveries must re-emit the airport account headers so clients can render
// expiry/traffic; when no upstream reported, the headers stay absent.
func TestDeliveryUserinfoHeaders(t *testing.T) {
	n1 := &model.ProxyNode{Tag: "A", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 8388, Method: "aes-128-gcm", Password: "p"}
	mock := &keyMock{
		nodes: []*model.ProxyNode{n1},
		keys:  map[string]*storage.SubKeyRef{"tk": {ID: "a"}},
		info:  &model.SubscriptionInfo{Upload: 1 << 30, Download: 2 << 30, Total: 100 << 30, Expire: 1735689600, UpdateInterval: 24, Reported: true},
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, mock, nil, nil).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tk&target=uri", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delivery: %d", rec.Code)
	}
	ui := rec.Header().Get("subscription-userinfo")
	want := "upload=1073741824; download=2147483648; total=107374182400; expire=1735689600"
	if ui != want {
		t.Fatalf("subscription-userinfo = %q want %q", ui, want)
	}
	if got := rec.Header().Get("profile-update-interval"); got != "24" {
		t.Fatalf("profile-update-interval = %q", got)
	}

	// Silent upstreams: no headers, delivery unaffected.
	mock.info = nil
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tk&target=uri", nil))
	if rec2.Code != http.StatusOK || rec2.Header().Get("subscription-userinfo") != "" {
		t.Fatalf("expected no userinfo header, got %d %q", rec2.Code, rec2.Header().Get("subscription-userinfo"))
	}
}
