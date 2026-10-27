package api_test

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/model"
)

type mockSubManager struct {
	expectedID    string
	expectedToken string
	nodes         []*model.ProxyNode
}

func (m *mockSubManager) GetSubscription(id string, token string) ([]*model.ProxyNode, error) {
	if id != m.expectedID || token != m.expectedToken {
		return nil, errors.New("forbidden")
	}
	return m.nodes, nil
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

	server := api.NewServer(cfg, mockSM, nil)
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

	t.Run("WebUI Index Route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "subrefine 控制台") {
			t.Fatalf("expected embedded html title, got %s", rec.Body.String())
		}
	})
}
