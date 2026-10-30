package emitter

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/core/model"
)

// allOpts keeps every managed inbound so the injection tests are unaffected
// by the option switch.
func allOpts() FullOptions { return FullOptions{Inbounds: []string{"mixed", "tun", "tproxy"}} }

func fullNodes() []*model.ProxyNode {
	return []*model.ProxyNode{
		{Tag: "HK-01", Type: model.ProtoShadowsocks, Server: "1.2.3.4", Port: 8388, Method: "aes-128-gcm", Password: "p"},
		{Tag: "US-LA", Type: model.ProtoTrojan, Server: "5.6.7.8", Port: 443, Password: "pw", Security: model.SecurityTLS},
		{Tag: "JP-RT", Type: model.ProtoVMess, Server: "9.9.9.9", Port: 443, UUID: "u", Security: model.SecurityTLS},
	}
}

const sbTemplate = `{
  "log": {"level": "info"},
  "inbounds": [{"type": "tun", "tag": "tun-in", "address": ["172.18.0.1/30"], "auto_route": true}],
  "outbounds": [
    {"type": "direct", "tag": "Direct"},
    {"type": "direct", "tag": "@subrefine"},
    {"type": "urltest", "tag": "Auto", "outbounds": ["@subrefine"], "url": "https://www.gstatic.com/generate_204"},
    {"type": "urltest", "tag": "HK", "outbounds": ["@subrefine:(?i)(HK|港)", "Direct"]},
    {"type": "selector", "tag": "Available", "outbounds": ["Auto", "HK", "@subrefine", "Direct"]}
  ],
  "route": {"final": "Available"}
}`

func TestSingboxFullTemplateLoadValidation(t *testing.T) {
	if _, err := LoadSingboxTemplate([]byte(`{"outbounds":[]}`)); err == nil {
		t.Fatal("template without injection point must fail")
	}
	if _, err := LoadSingboxTemplate([]byte(`{"outbounds":[
		{"type":"direct","tag":"@subrefine"},{"type":"direct","tag":"@subrefine"}]}`)); err == nil {
		t.Fatal("two injection points must fail")
	}
	if _, err := LoadSingboxTemplate([]byte(`{"outbounds":[
		{"type":"direct","tag":"@subrefine"},
		{"type":"selector","tag":"G","outbounds":["@subrefine:("]}]}`)); err == nil {
		t.Fatal("invalid marker regex must fail at load")
	}
	if _, err := LoadSingboxTemplate([]byte(sbTemplate)); err != nil {
		t.Fatalf("valid template rejected: %v", err)
	}
}

func TestSingboxFullRenderInjectsAndExpands(t *testing.T) {
	tmpl, err := LoadSingboxTemplate([]byte(sbTemplate))
	if err != nil {
		t.Fatal(err)
	}
	data, err := tmpl.Render(fullNodes(), allOpts())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Outbounds []struct {
			Type      string   `json:"type"`
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		} `json:"outbounds"`
		Route map[string]any `json:"route"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("render is not valid json: %v", err)
	}
	if strings.Contains(string(data), "@subrefine") {
		t.Fatalf("marker leaked into output: %s", data)
	}

	byTag := map[string]int{}
	var order []string
	for _, ob := range doc.Outbounds {
		byTag[ob.Tag]++
		order = append(order, ob.Tag)
	}
	// Injection replaced the marker outbound in place: nodes sit between
	// Direct and Auto, marker gone.
	if byTag["@subrefine"] != 0 {
		t.Fatal("injection marker outbound survived")
	}
	for _, want := range []string{"HK-01", "US-LA", "JP-RT"} {
		if byTag[want] != 1 {
			t.Fatalf("node outbound %s missing or duplicated: %v", want, order)
		}
	}
	if order[0] != "Direct" || order[1] != "HK-01" || order[4] != "Auto" {
		t.Fatalf("injection not in place: %v", order)
	}

	var auto, hk, avail []string
	for _, ob := range doc.Outbounds {
		switch ob.Tag {
		case "Auto":
			auto = ob.Outbounds
		case "HK":
			hk = ob.Outbounds
		case "Available":
			avail = ob.Outbounds
		}
	}
	if strings.Join(auto, ",") != "HK-01,US-LA,JP-RT" {
		t.Fatalf("bare marker expansion wrong: %v", auto)
	}
	// regex group keeps literal entries after the matched subset
	if strings.Join(hk, ",") != "HK-01,Direct" {
		t.Fatalf("regex expansion wrong: %v", hk)
	}
	if strings.Join(avail, ",") != "Auto,HK,HK-01,US-LA,JP-RT,Direct" {
		t.Fatalf("mixed group expansion wrong: %v", avail)
	}
}

func TestSingboxFullRenderRegexFallbackAndGuards(t *testing.T) {
	tmpl, err := LoadSingboxTemplate([]byte(sbTemplate))
	if err != nil {
		t.Fatal(err)
	}
	// No node matches the HK regex: the group degrades to all nodes instead
	// of shipping an empty (client-rejected) urltest.
	only := []*model.ProxyNode{{Tag: "FRA-01", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 1, Method: "aes-128-gcm", Password: "p"}}
	data, err := tmpl.Render(only, allOpts())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tag": "HK"`) || !strings.Contains(string(data), "FRA-01") {
		t.Fatalf("fallback group missing: %s", data)
	}
	var doc struct {
		Outbounds []struct {
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		} `json:"outbounds"`
	}
	json.Unmarshal(data, &doc)
	for _, ob := range doc.Outbounds {
		if ob.Tag == "HK" && strings.Join(ob.Outbounds, ",") != "FRA-01,Direct" {
			t.Fatalf("regex-with-no-match must fall back to all nodes: %v", ob.Outbounds)
		}
	}

	// Zero deliverable nodes is an explicit error, never an empty config.
	if _, err := tmpl.Render(nil, allOpts()); !errors.Is(err, ErrNoDeliverableNodes) {
		t.Fatalf("empty node set: %v, want ErrNoDeliverableNodes", err)
	}
	// WG-only node set is likewise undeliverable to sing-box.
	wgOnly := []*model.ProxyNode{{Tag: "WG", Type: model.ProtoWireGuard, Server: "1.2.3.4", Port: 51820, PrivateKey: "k", PeerPubKey: "p"}}
	if _, err := tmpl.Render(wgOnly, allOpts()); !errors.Is(err, ErrNoDeliverableNodes) {
		t.Fatalf("wg-only node set: %v, want ErrNoDeliverableNodes", err)
	}
}

func TestSingboxFullRenderRejectsStrayMarker(t *testing.T) {
	// A marker in route (no list expansion happens there) must fail loudly.
	tmpl, err := LoadSingboxTemplate([]byte(`{"outbounds":[{"type":"direct","tag":"@subrefine"}],"route":{"final":"@subrefine"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpl.Render(fullNodes(), allOpts()); err == nil || !strings.Contains(err.Error(), "outside outbounds group lists") {
		t.Fatalf("stray marker must error, got %v", err)
	}
}

const clashTemplate = `log-level: info
mixed-port: 7890
tun:
  enable: true
  stack: system
proxies:
  - name: Direct-Local
    type: direct
  - name: "@subrefine"
proxy-groups:
  - name: AUTO
    type: url-test
    proxies: ["@subrefine"]
  - name: HK
    type: select
    proxies: ["@subrefine:(?i)(HK|港)", "AUTO"]
rules:
  - GEOSITE,cn,DIRECT
  - MATCH,HK
`

func TestClashFullTemplateRender(t *testing.T) {
	tmpl, err := LoadClashFullTemplate([]byte(clashTemplate))
	if err != nil {
		t.Fatalf("valid clash template rejected: %v", err)
	}
	nodes := fullNodes()
	nodes = append(nodes, &model.ProxyNode{Tag: "WG-DE", Type: model.ProtoWireGuard, Server: "1.2.3.4", Port: 51820, PrivateKey: "a", PeerPubKey: "b", LocalAddr: []string{"10.0.0.2/32"}})
	data, err := tmpl.Render(nodes, allOpts())
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, "@subrefine") {
		t.Fatalf("marker leaked: %s", out)
	}
	// clash carries wireguard, unlike the sing-box target.
	for _, want := range []string{"name: HK-01", "name: WG-DE", "type: wireguard"} {
		if !strings.Contains(out, want) {
			t.Fatalf("clash full output missing %q:\n%s", want, out)
		}
	}
	// Group expansion: AUTO all four, HK only HK-01 + literal AUTO.
	for _, want := range []string{"- HK-01", "- AUTO"} {
		if !strings.Contains(out, want) {
			t.Fatalf("clash group expansion missing %q:\n%s", want, out)
		}
	}
}

func TestClashFullTemplateLoadValidation(t *testing.T) {
	if _, err := LoadClashFullTemplate([]byte("proxies: []\n")); err == nil {
		t.Fatal("missing injection must fail")
	}
	if _, err := LoadClashFullTemplate([]byte("proxy-groups:\n  - {name: G, type: select, proxies: [\"@subrefine:(\"]}\nproxies:\n  - {name: \"@subrefine\"}\n")); err == nil {
		t.Fatal("invalid marker regex must fail at load")
	}
}

const sbMultiTemplate = `{
  "dns": {"servers": [], "strategy": "ipv4_only"},
  "inbounds": [
    {"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 7890},
    {"type": "tun", "tag": "tun-in", "address": ["172.18.0.1/30"], "auto_route": true},
    {"type": "tproxy", "tag": "tp-in", "listen": "127.0.0.1", "listen_port": 9000},
    {"type": "redirect", "tag": "keep-me", "listen": "127.0.0.1", "listen_port": 9300}
  ],
  "outbounds": [
    {"type": "direct", "tag": "@subrefine"},
    {"type": "selector", "tag": "G", "outbounds": ["@subrefine"]}
  ]
}`

func TestSingboxFullRenderFiltersInbounds(t *testing.T) {
	tmpl, err := LoadSingboxTemplate([]byte(sbMultiTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tmpl.InboundOptions(), ","); got != "mixed,tproxy,tun" {
		t.Fatalf("InboundOptions = %q, want mixed,tproxy,tun (redirect is unmanaged)", got)
	}

	data, err := tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Inbounds []struct {
			Type string `json:"type"`
			Tag  string `json:"tag"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	kept := map[string]bool{}
	for _, in := range doc.Inbounds {
		kept[in.Type] = true
	}
	if !kept["tun"] {
		t.Fatal("selected tun dropped")
	}
	if kept["mixed"] || kept["tproxy"] {
		t.Fatalf("unselected managed inbounds survived: %v", kept)
	}
	if !kept["redirect"] {
		t.Fatal("unmanaged (custom) inbound must always be kept")
	}
}

func TestSingboxFullRenderIPv6(t *testing.T) {
	tmpl, _ := LoadSingboxTemplate([]byte(sbMultiTemplate))
	on := true
	off := false
	data, _ := tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}, IPv6: &on})
	if !strings.Contains(string(data), `"strategy": "prefer_ipv4"`) {
		t.Fatalf("ipv6 on must set prefer_ipv4: %s", data)
	}
	data, _ = tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}, IPv6: &off})
	if !strings.Contains(string(data), `"strategy": "ipv4_only"`) {
		t.Fatalf("ipv6 off must set ipv4_only: %s", data)
	}
	// nil leaves the template's own strategy untouched.
	data, _ = tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}})
	if !strings.Contains(string(data), `"strategy": "ipv4_only"`) {
		t.Fatalf("nil ipv6 must not change strategy: %s", data)
	}
}

const clashMultiTemplate = `log-level: info
ipv6: true
mixed-port: 7890
tproxy-port: 9000
redir-port: 9300
tun:
  enable: true
  stack: system
dns:
  enable: true
  ipv6: true
proxies:
  - name: "@subrefine"
proxy-groups:
  - name: AUTO
    type: url-test
    proxies: ["@subrefine"]
rules:
  - MATCH,AUTO
`

func TestClashFullRenderFiltersInbounds(t *testing.T) {
	tmpl, err := LoadClashFullTemplate([]byte(clashMultiTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tmpl.InboundOptions(), ","); got != "mixed,tproxy,tun" {
		t.Fatalf("InboundOptions = %q", got)
	}
	data, err := tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}})
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "tun:") {
		t.Fatal("selected tun removed")
	}
	for _, banned := range []string{"mixed-port:", "tproxy-port:", "redir-port:"} {
		if strings.Contains(out, banned) {
			t.Fatalf("unselected listener key %q survived:\n%s", banned, out)
		}
	}
}

func TestClashFullRenderIPv6(t *testing.T) {
	tmpl, _ := LoadClashFullTemplate([]byte(clashMultiTemplate))
	off := false
	data, _ := tmpl.Render(fullNodes(), FullOptions{Inbounds: []string{"tun"}, IPv6: &off})
	out := string(data)
	if !strings.Contains(out, "ipv6: false") {
		t.Fatalf("ipv6 off must set top-level ipv6:false:\n%s", out)
	}
	// dns.ipv6 must follow too.
	if strings.Count(out, "ipv6: false") < 2 {
		t.Fatalf("dns.ipv6 must also be false:\n%s", out)
	}
}
