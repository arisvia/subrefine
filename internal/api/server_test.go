package api_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/storage"
)

type mockSubManager struct {
	expectedID    string
	expectedToken string
	nodes         []*model.ProxyNode
	disabled      []string // per-key blacklist the mock's Resolve key carries
	info          *model.SubscriptionInfo
	sources       []*model.SourceInfo
	invalidated   []string
	sbMode        string
	clMode        string
}

func (m *mockSubManager) GetSubscription(id string, token string) ([]*model.ProxyNode, error) {
	if id != m.expectedID || token != m.expectedToken {
		return nil, errors.New("forbidden")
	}
	return m.nodes, nil
}

func (m *mockSubManager) Resolve(id, token string) (*storage.Subscription, *storage.SubKey, []*model.ProxyNode, error) {
	if id != m.expectedID || token != m.expectedToken {
		return nil, nil, nil, errors.New("forbidden")
	}
	sub := &storage.Subscription{ID: id, Token: token, Enabled: true}
	key := &storage.SubKey{ID: "default", Name: "Default", Token: token, Disabled: m.disabled}
	return sub, key, m.nodes, nil
}

func (m *mockSubManager) Invalidate(id string) {
	m.invalidated = append(m.invalidated, id)
}

func (m *mockSubManager) FullModes(id string) (string, string) {
	return m.sbMode, m.clMode
}

func (m *mockSubManager) UserInfo(id string) (*model.SubscriptionInfo, []*model.SourceInfo, error) {
	if id != m.expectedID {
		return nil, nil, errors.New("not found")
	}
	return m.info, m.sources, nil
}

func TestAPIServer(t *testing.T) {
	cfg := &config.Config{
		Host:       "127.0.0.1",
		Port:       3000,
		AdminToken: "my_admin_secret",
	}

	mockSM := &mockSubManager{
		expectedID:    "sub1",
		expectedToken: "tok123",
		nodes: []*model.ProxyNode{
			{
				Tag:      "Node1",
				Type:     model.ProtoShadowsocks,
				Server:   "1.2.3.4",
				Port:     8388,
				Method:   "aes-128-gcm",
				Password: "pass",
			},
		},
	}

	server := api.NewServer(cfg, mockSM, nil, nil)
	handler := server.Handler()

	t.Run("Healthz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Admin Auth Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/info", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Admin Auth Authorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/info", nil)
		req.Header.Set("Authorization", "Bearer my_admin_secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("Subscribe Forbidden On Wrong Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/s/sub1?token=wrong", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("Subscribe Target Singbox via UA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123", nil)
		req.Header.Set("User-Agent", "sing-box/1.15.0")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"outbounds"`) {
			t.Fatalf("expected sing-box json outbounds, got %s", rec.Body.String())
		}
	})

	t.Run("Subscribe Target Clash via Param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=clash", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "proxies:") {
			t.Fatalf("expected clash yaml proxies, got %s", rec.Body.String())
		}
	})

	t.Run("Subscribe Target Base64 Default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		decoded, err := base64.StdEncoding.DecodeString(rec.Body.String())
		if err != nil {
			t.Fatalf("expected valid base64, got err %v", err)
		}
		if !strings.Contains(string(decoded), "ss://") {
			t.Fatalf("expected decoded content to contain ss://, got %s", string(decoded))
		}
	})

	t.Run("Version Is Reported", func(t *testing.T) {
		api.Version = "v9.9.9-test"
		defer func() { api.Version = "dev" }()

		req := httptest.NewRequest(http.MethodGet, "/api/admin/info", nil)
		req.Header.Set("Authorization", "Bearer my_admin_secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("bad json: %v (%s)", err, rec.Body.String())
		}
		if body["version"] != "v9.9.9-test" {
			t.Fatalf("admin info must report the stamped build version, got %v", body["version"])
		}
	})

	t.Run("Console Shell Is Mounted At Root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("expected an HTML shell, got Content-Type %q", ct)
		}
		// The shell must never be cached, or a redeployed bundle would only
		// reach clients that dropped their cache.
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Fatalf("expected no-store shell, got Cache-Control %q", cc)
		}
	})
}
