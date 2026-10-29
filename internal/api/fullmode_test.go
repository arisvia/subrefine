package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/model"
)

// fakeRegistry serves the real template machinery without touching disk.
type fakeRegistry struct {
	sb map[string]*emitter.SingboxTemplate
	cl map[string]*emitter.ClashFullTemplate
}

func (f *fakeRegistry) SingboxTemplate(mode string) *emitter.SingboxTemplate { return f.sb[mode] }
func (f *fakeRegistry) ClashTemplate(mode string) *emitter.ClashFullTemplate { return f.cl[mode] }
func (f *fakeRegistry) Modes() (singbox, clash []string) {
	for k := range f.sb {
		singbox = append(singbox, k)
	}
	for k := range f.cl {
		clash = append(clash, k)
	}
	return
}

const sbTemplate = `{
  "log": {"level": "info"},
  "dns": {"servers": [{"type": "local", "tag": "local", "server": "223.5.5.5"}]},
  "inbounds": [{"type": "tun", "tag": "tun-in", "address": ["172.18.0.1/30"], "auto_route": true}],
  "outbounds": [
    {"type": "direct", "tag": "Direct"},
    {"type": "direct", "tag": "@subrefine"},
    {"type": "urltest", "tag": "Auto", "outbounds": ["@subrefine"]}
  ],
  "route": {"final": "Auto"}
}`

const clTemplate = `mixed-port: 7890
proxies:
  - {name: Local, type: direct}
  - {name: "@subrefine"}
proxy-groups:
  - {name: AUTO, type: url-test, proxies: ["@subrefine"]}
rules:
  - MATCH,AUTO
`

func fullTestRegistry(t *testing.T) *fakeRegistry {
	t.Helper()
	sb, err := emitter.LoadSingboxTemplate([]byte(sbTemplate))
	if err != nil {
		t.Fatal(err)
	}
	cl, err := emitter.LoadClashFullTemplate([]byte(clTemplate))
	if err != nil {
		t.Fatal(err)
	}
	return &fakeRegistry{
		sb: map[string]*emitter.SingboxTemplate{"tun": sb},
		cl: map[string]*emitter.ClashFullTemplate{"tun": cl},
	}
}

func fullTestNodes() []*model.ProxyNode {
	return []*model.ProxyNode{
		{Tag: "HK-01", Type: model.ProtoShadowsocks, Server: "1.2.3.4", Port: 8388, Method: "aes-128-gcm", Password: "p"},
		{Tag: "WG-DE", Type: model.ProtoWireGuard, Server: "1.2.3.4", Port: 51820, PrivateKey: "a", PeerPubKey: "b", LocalAddr: []string{"10.0.0.2/32"}},
	}
}

func newFullModeServer(t *testing.T) http.Handler {
	t.Helper()
	mock := &mockSubManager{
		expectedID:    "sub1",
		expectedToken: "tok123",
		nodes:         fullTestNodes(),
	}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "adm"}
	return api.NewServer(cfg, mock, nil, fullTestRegistry(t)).Handler()
}

func TestSingboxFullDelivery(t *testing.T) {
	h := newFullModeServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&mode=tun", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Subrefine-Full"); got != "singbox.tun" {
		t.Fatalf("X-Subrefine-Full = %q", got)
	}
	// WG still dropped from sing-box and announced, even in full mode.
	if got := rec.Header().Get("X-Subrefine-Skipped"); got != "wireguard=1" {
		t.Fatalf("X-Subrefine-Skipped = %q", got)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dns", "route", "inbounds", "log"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("full config missing %q section: %s", key, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "@subrefine") {
		t.Fatal("marker leaked")
	}

	// Without a mode parameter the fragment is the default: nothing is bound
	// to the subscription; mode=off is the explicit fragment choice.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box", nil))
	if rec.Header().Get("X-Subrefine-Full") != "" || strings.Contains(rec.Body.String(), `"route"`) {
		t.Fatalf("bare delivery must serve the fragment: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&mode=off", nil))
	if rec.Header().Get("X-Subrefine-Full") != "" || strings.Contains(rec.Body.String(), `"route"`) {
		t.Fatalf("mode=off must serve the fragment: %s", rec.Body.String())
	}

	// Unknown mode: fragment + explicit header, never a broken full config.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&mode=bogus", nil))
	if rec.Header().Get("X-Subrefine-Full") != "unknown-mode" || !strings.Contains(rec.Body.String(), `"outbounds"`) {
		t.Fatalf("unknown mode: header=%q body=%s", rec.Header().Get("X-Subrefine-Full"), rec.Body.String())
	}
}

func TestClashFullDelivery(t *testing.T) {
	h := newFullModeServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=clash&mode=tun", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("X-Subrefine-Full"); got != "clash.tun" {
		t.Fatalf("X-Subrefine-Full = %q", got)
	}
	body := rec.Body.String()
	// clash full carries wireguard and the template's mixed-port.
	if !strings.Contains(body, "mixed-port") || !strings.Contains(body, "type: wireguard") || strings.Contains(body, "@subrefine") {
		t.Fatalf("clash full body wrong:\n%s", body)
	}
}

func TestFullDeliveryEmptyNodes(t *testing.T) {
	mock := &mockSubManager{expectedID: "sub1", expectedToken: "tok123"}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "adm"}
	h := api.NewServer(cfg, mock, nil, fullTestRegistry(t)).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&mode=tun", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Subrefine-Full") != "empty-nodes" {
		t.Fatalf("empty nodes: %d header=%q", rec.Code, rec.Header().Get("X-Subrefine-Full"))
	}
}

func TestAdminTemplatesListsModes(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "adm"}
	h := api.NewServer(cfg, &mockSubManager{}, nil, fullTestRegistry(t)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/templates", nil)
	req.Header.Set("Authorization", "Bearer adm")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Singbox []string `json:"singbox"`
		Clash   []string `json:"clash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Singbox) != 1 || body.Singbox[0] != "tun" || len(body.Clash) != 1 || body.Clash[0] != "tun" {
		t.Fatalf("modes wrong: %+v", body)
	}

	// Unauthenticated must not leak the mode list.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/templates", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status %d", rec.Code)
	}
}
