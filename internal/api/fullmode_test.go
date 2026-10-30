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
	sb *emitter.SingboxTemplate
	cl *emitter.ClashFullTemplate
}

func (f *fakeRegistry) SingboxTemplate() *emitter.SingboxTemplate { return f.sb }
func (f *fakeRegistry) ClashTemplate() *emitter.ClashFullTemplate { return f.cl }
func (f *fakeRegistry) Options() (sb, cl emitter.FamilyOptions) {
	sb, cl = emitter.FamilyOptions{Inbounds: []string{}}, emitter.FamilyOptions{Inbounds: []string{}}
	if f.sb != nil {
		sb.Available = true
		sb.Inbounds = f.sb.InboundOptions()
	}
	if f.cl != nil {
		cl.Available = true
		cl.Inbounds = f.cl.InboundOptions()
	}
	return
}

const sbTemplate = `{
  "log": {"level": "info"},
  "dns": {"servers": [{"type": "local", "tag": "local", "server": "223.5.5.5"}], "strategy": "prefer_ipv4"},
  "inbounds": [
    {"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 7890},
    {"type": "tun", "tag": "tun-in", "address": ["172.18.0.1/30"], "auto_route": true}
  ],
  "outbounds": [
    {"type": "direct", "tag": "Direct"},
    {"type": "direct", "tag": "@subrefine"},
    {"type": "urltest", "tag": "Auto", "outbounds": ["@subrefine"]}
  ],
  "route": {"final": "Auto"}
}`

const clTemplate = `mixed-port: 7890
tun:
  enable: true
  stack: system
dns:
  enable: true
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
	return &fakeRegistry{sb: sb, cl: cl}
}

func fullTestNodes() []*model.ProxyNode {
	return []*model.ProxyNode{
		{Tag: "HK-01", Type: model.ProtoShadowsocks, Server: "1.2.3.4", Port: 8388, Method: "aes-128-gcm", Password: "p"},
		{Tag: "WG-DE", Type: model.ProtoWireGuard, Server: "1.2.3.4", Port: 51820, PrivateKey: "a", PeerPubKey: "b", LocalAddr: []string{"10.0.0.2/32"}},
	}
}

func newFullModeServer(t *testing.T) http.Handler {
	t.Helper()
	mock := &mockSubManager{expectedID: "sub1", expectedToken: "tok123", nodes: fullTestNodes()}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "adm"}
	return api.NewServer(cfg, mock, nil, fullTestRegistry(t)).Handler()
}

func inboundTypes(t *testing.T, body []byte) map[string]bool {
	t.Helper()
	var doc struct {
		Inbounds []struct {
			Type string `json:"type"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("not json: %v", err)
	}
	got := map[string]bool{}
	for _, in := range doc.Inbounds {
		got[in.Type] = true
	}
	return got
}

func TestSingboxFullDelivery(t *testing.T) {
	h := newFullModeServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&full=1&inbounds=tun", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Subrefine-Full"); got != "singbox" {
		t.Fatalf("X-Subrefine-Full = %q", got)
	}
	if got := rec.Header().Get("X-Subrefine-Skipped"); got != "wireguard=1" {
		t.Fatalf("X-Subrefine-Skipped = %q", got)
	}
	body := rec.Body.String()
	for _, key := range []string{"dns", "route", "inbounds", "log"} {
		if !strings.Contains(body, `"`+key+`"`) {
			t.Fatalf("full config missing %q: %s", key, body)
		}
	}
	if strings.Contains(body, "@subrefine") {
		t.Fatal("marker leaked")
	}
	types := inboundTypes(t, rec.Body.Bytes())
	if !types["tun"] || types["mixed"] {
		t.Fatalf("inbound filter wrong: %v", types)
	}
}

func TestSingboxFullDefaultKeepsAllInbounds(t *testing.T) {
	h := newFullModeServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&full=1", nil))
	types := inboundTypes(t, rec.Body.Bytes())
	if !types["tun"] || !types["mixed"] {
		t.Fatalf("bare full=1 must keep every declared inbound: %v", types)
	}
}

func TestSingboxFullIPv6(t *testing.T) {
	h := newFullModeServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&full=1&inbounds=tun&ipv6=0", nil))
	if !strings.Contains(rec.Body.String(), `"strategy": "ipv4_only"`) {
		t.Fatalf("ipv6=0 must force ipv4_only: %s", rec.Body.String())
	}
}

func TestClashFullDelivery(t *testing.T) {
	h := newFullModeServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=clash&full=1&inbounds=tun&ipv6=0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("X-Subrefine-Full"); got != "clash" {
		t.Fatalf("X-Subrefine-Full = %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "type: wireguard") || strings.Contains(body, "@subrefine") {
		t.Fatalf("clash full body wrong:\n%s", body)
	}
	if !strings.Contains(body, "tun:") || strings.Contains(body, "mixed-port:") {
		t.Fatalf("clash listener filter wrong:\n%s", body)
	}
	if !strings.Contains(body, "ipv6: false") {
		t.Fatalf("clash ipv6=0 wrong:\n%s", body)
	}
}

func TestFragmentDefault(t *testing.T) {
	h := newFullModeServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box", nil))
	if rec.Header().Get("X-Subrefine-Full") != "" || !strings.Contains(rec.Body.String(), `"outbounds"`) {
		t.Fatalf("bare delivery must serve the fragment: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=clash", nil))
	if b := rec.Body.String(); strings.Contains(b, "proxy-groups") || strings.Contains(b, "rules:") || !strings.Contains(b, "proxies:") {
		t.Fatalf("clash fragment must be nodes-only:\n%s", b)
	}
}

func TestFullDeliveryEmptyNodes(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "adm"}
	h := api.NewServer(cfg, &mockSubManager{expectedID: "sub1", expectedToken: "tok123"}, nil, fullTestRegistry(t)).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/s/sub1?token=tok123&target=sing-box&full=1", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Subrefine-Full") != "empty-nodes" {
		t.Fatalf("empty nodes: %d header=%q", rec.Code, rec.Header().Get("X-Subrefine-Full"))
	}
}

func TestAdminTemplatesReportsOptions(t *testing.T) {
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
		Singbox emitter.FamilyOptions `json:"singbox"`
		Clash   emitter.FamilyOptions `json:"clash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Singbox.Available || strings.Join(body.Singbox.Inbounds, ",") != "mixed,tun" {
		t.Fatalf("singbox options wrong: %+v", body.Singbox)
	}
	if !body.Clash.Available || strings.Join(body.Clash.Inbounds, ",") != "mixed,tun" {
		t.Fatalf("clash options wrong: %+v", body.Clash)
	}
}
