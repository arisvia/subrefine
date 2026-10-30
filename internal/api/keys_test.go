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

// keyMock authenticates several delivery tokens, each attached with its own
// (subscription, key) blacklist.
type keyMock struct {
	nodes   []*model.ProxyNode
	keys    map[string]*storage.SubKeyRef
	info    *model.SubscriptionInfo
	sources []*model.SourceInfo
}

func (m *keyMock) GetSubscription(id, token string) ([]*model.ProxyNode, error) {
	if _, ok := m.keys[token]; !ok {
		return nil, errors.New("forbidden")
	}
	return m.nodes, nil
}

func (m *keyMock) Resolve(id, token string) (*storage.Subscription, *storage.SubKeyRef, []*model.ProxyNode, error) {
	ref, ok := m.keys[token]
	if !ok {
		return nil, nil, nil, errors.New("forbidden")
	}
	return &storage.Subscription{ID: id, Enabled: true}, ref, m.nodes, nil
}

func (m *keyMock) Invalidate(string) {}
func (m *keyMock) FullModes(string) (string, string) {
	return "", ""
}
func (m *keyMock) UserInfo(string) (*model.SubscriptionInfo, []*model.SourceInfo, error) {
	return m.info, m.sources, nil
}

// Different tokens must deliver different node sets: the per-key blacklist is
// applied at request time, on top of the shared full node cache.
func TestKeyDeliveryBlacklist(t *testing.T) {
	n1 := &model.ProxyNode{Tag: "Keep", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 8388, Method: "aes-128-gcm", Password: "p"}
	n2 := &model.ProxyNode{Tag: "Hide", Type: model.ProtoTrojan, Server: "2.2.2.2", Port: 443, Password: "tp"}
	mock := &keyMock{
		nodes: []*model.ProxyNode{n1, n2},
		keys: map[string]*storage.SubKeyRef{
			"tok-full":  {ID: "a"},
			"tok-minus": {ID: "b", Disabled: []string{n2.Identity()}},
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
		Count int `json:"count"`
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

// The global key API: create (generated token), attach by reference through a
// subscription save, reject unknown references, and reject duplicate tokens.
func TestGlobalKeyLifecycle(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, &mockSubManager{}, store, nil).Handler()

	admin := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer admin")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := admin(http.MethodPost, "/api/admin/keys", `{"name":"Bob"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create key: %d %s", rec.Code, rec.Body.String())
	}
	var bob struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bob); err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if len(bob.Token) != 32 || bob.ID == "" {
		t.Fatalf("generated key fields wrong: %+v", bob)
	}

	// Duplicate token rejected with 409.
	if code := admin(http.MethodPost, "/api/admin/keys", `{"name":"Clone","token":"`+bob.Token+`"}`).Code; code != http.StatusConflict {
		t.Fatalf("duplicate token must be 409, got %d", code)
	}

	// Attach Bob to a subscription by reference.
	rec = admin(http.MethodPost, "/api/admin/subscriptions",
		`{"id":"feed","enabled":true,"sources":["x"],"keys":[{"id":"`+bob.ID+`"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("attach: %d %s", rec.Code, rec.Body.String())
	}
	saved, _ := store.Get("feed")
	if saved.Token != bob.Token {
		t.Fatalf("primary token must mirror attached key: %q vs %q", saved.Token, bob.Token)
	}

	// Unknown reference rejected with 400, not 500.
	if code := admin(http.MethodPost, "/api/admin/subscriptions",
		`{"id":"bad","enabled":true,"sources":["x"],"keys":[{"id":"ghost"}]}`).Code; code != http.StatusBadRequest {
		t.Fatalf("unknown key ref must be 400, got %d", code)
	}

	// Listing surfaces usage so the console can show blast radius.
	rec = admin(http.MethodGet, "/api/admin/keys", "")
	var listed []struct {
		ID    string   `json:"id"`
		InUse []string `json:"in_use"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != bob.ID || len(listed[0].InUse) != 1 || listed[0].InUse[0] != "feed" {
		t.Fatalf("usage not reported: %+v", listed)
	}

	// Deleting an attached key is blocked until forced with cascade.
	if code := admin(http.MethodDelete, "/api/admin/keys/"+bob.ID, "").Code; code != http.StatusConflict {
		t.Fatalf("in-use delete must be 409, got %d", code)
	}
	if code := admin(http.MethodDelete, "/api/admin/keys/"+bob.ID+"?cascade=1", "").Code; code != http.StatusOK {
		t.Fatalf("cascade delete must be 200, got %d", code)
	}
	after, _ := store.Get("feed")
	if after.Token == bob.Token || len(after.Token) != 32 {
		t.Fatalf("cascade must issue a fresh default key, got %q", after.Token)
	}
}

// Rotating a key's token through the API re-syncs the mirrored primary token
// on every attaching subscription, so the console shows the new link.
func TestKeyRotationPropagates(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "admin"}
	h := api.NewServer(cfg, &mockSubManager{}, store, nil).Handler()
	admin := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer admin")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	k := &storage.DeliveryKey{Name: "Alice", Token: "old-token"}
	if err := store.SaveKey(k); err != nil {
		t.Fatalf("seed key: %v", err)
	}
	if err := store.Save(&storage.Subscription{ID: "feed", Enabled: true, Sources: []string{"x"},
		Keys: []storage.SubKeyRef{{ID: k.ID}}}); err != nil {
		t.Fatalf("seed sub: %v", err)
	}

	rec := admin(http.MethodPut, "/api/admin/keys/"+k.ID, `{"token":"rotated-99"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body.String())
	}
	sub, _ := store.Get("feed")
	if sub.Token != "rotated-99" {
		t.Fatalf("rotation must re-sync mirror, got %q", sub.Token)
	}
}
