package api_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/storage"
)

// keyMock authenticates several delivery tokens, each with its own blacklist.
type keyMock struct {
	nodes []*model.ProxyNode
	keys  map[string]*storage.SubKey
}

func (m *keyMock) GetSubscription(id, token string) ([]*model.ProxyNode, error) {
	if _, ok := m.keys[token]; !ok {
		return nil, errors.New("forbidden")
	}
	return m.nodes, nil
}

func (m *keyMock) Resolve(id, token string) (*storage.Subscription, *storage.SubKey, []*model.ProxyNode, error) {
	k, ok := m.keys[token]
	if !ok {
		return nil, nil, nil, errors.New("forbidden")
	}
	return &storage.Subscription{ID: id, Enabled: true}, k, m.nodes, nil
}

func (m *keyMock) Invalidate(string) {}
func (m *keyMock) FullModes(string) (string, string) {
	return "", ""
}

// Different tokens must deliver different node sets: the per-key blacklist is
// applied at request time, on top of the shared full node cache.
func TestKeyDeliveryBlacklist(t *testing.T) {
	n1 := &model.ProxyNode{Tag: "Keep", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 8388, Method: "aes-128-gcm", Password: "p"}
	n2 := &model.ProxyNode{Tag: "Hide", Type: model.ProtoTrojan, Server: "2.2.2.2", Port: 443, Password: "tp"}
	mock := &keyMock{
		nodes: []*model.ProxyNode{n1, n2},
		keys: map[string]*storage.SubKey{
			"tok-full":  {ID: "a", Token: "tok-full"},
			"tok-minus": {ID: "b", Token: "tok-minus", Disabled: []string{n2.Identity()}},
		},
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, mock, nil, nil).Handler()

	deliver := func(token string) *http.Response {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token="+token+"&target=uri", nil))
		return rec.Result()
	}

	full := deliver("tok-full")
	if full.StatusCode != http.StatusOK {
		t.Fatalf("primary token rejected: %d", full.StatusCode)
	}
	body, _ := io.ReadAll(full.Body)
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		t.Fatalf("uri body not base64: %v", err)
	}
	if !strings.Contains(string(decoded), "1.1.1.1:8388") || !strings.Contains(string(decoded), "2.2.2.2:443") {
		t.Fatalf("full key lost a node:\n%s", decoded)
	}

	got := deliver("tok-minus")
	body, _ = io.ReadAll(got.Body)
	decoded, _ = base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if !strings.Contains(string(decoded), "1.1.1.1:8388") {
		t.Fatalf("blacklist dropped the wrong node:\n%s", decoded)
	}
	if strings.Contains(string(decoded), "2.2.2.2:443") {
		t.Fatalf("blacklisted node still delivered:\n%s", decoded)
	}

	if code := deliver("nope").StatusCode; code != http.StatusForbidden {
		t.Fatalf("unknown token must be 403, got %d", code)
	}
}

// The node preview must expose index-aligned identities so the console can
// record per-key blacklists that survive upstream renames.
func TestPreviewExposesIdentities(t *testing.T) {
	n1 := &model.ProxyNode{Tag: "A", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 8388, Method: "aes-128-gcm", Password: "p"}
	n2 := &model.ProxyNode{Tag: "B", Type: model.ProtoTrojan, Server: "2.2.2.2", Port: 443, Password: "tp"}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save(&storage.Subscription{ID: "sub1", Token: "tok123", Enabled: true, Sources: []string{"x"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	mock := &mockSubManager{expectedID: "sub1", expectedToken: "tok123", nodes: []*model.ProxyNode{n1, n2}}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, mock, store, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions/sub1/nodes", nil)
	req.Header.Set("Authorization", "Bearer admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Count int      `json:"count"`
		Nodes []struct {
			Tag string `json:"tag"`
		} `json:"nodes"`
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 2 || len(out.IDs) != 2 {
		t.Fatalf("ids must align with nodes: %+v", out)
	}
	if out.IDs[0] != n1.Identity() || out.IDs[1] != n2.Identity() {
		t.Fatalf("ids not index-aligned identities: %v", out.IDs)
	}
}

// Saving through the admin API normalizes keys (generated tokens, synced
// primary) and rejects ambiguous duplicates with 400, not 500.
func TestSaveKeyNormalizationAndDuplicates(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, &mockSubManager{}, store, nil).Handler()

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/subscriptions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer admin")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := post(`{"id":"gen","enabled":true,"sources":["x"],"keys":[{"name":"A"},{"name":"B"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("save with empty tokens: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Sub storage.Subscription `json:"sub"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sub.Keys) != 2 || len(resp.Sub.Keys[0].Token) != 32 || resp.Sub.Keys[0].Token == resp.Sub.Keys[1].Token {
		t.Fatalf("keys not generated distinctly: %+v", resp.Sub.Keys)
	}
	if resp.Sub.Token != resp.Sub.Keys[0].Token {
		t.Fatal("primary token not synced to keys[0]")
	}

	rec = post(`{"id":"dup","enabled":true,"sources":["x"],"keys":[{"token":"same"},{"token":"same"}]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate tokens must be 400, got %d %s", rec.Code, rec.Body.String())
	}
}
