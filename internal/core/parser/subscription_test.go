package parser_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/fetcher"
	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/core/parser"
)

func sampleNodes() []*model.ProxyNode {
	return []*model.ProxyNode{
		{
			Tag:      "SS-Test",
			Type:     model.ProtoShadowsocks,
			Server:   "1.1.1.1",
			Port:     8388,
			Method:   "aes-128-gcm",
			Password: "pass",
		},
		{
			Tag:         "VLESS-Reality",
			Type:        model.ProtoVLESS,
			Server:      "example.org",
			Port:        443,
			UUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Flow:        "xtls-rprx-vision",
			Security:    model.SecurityReality,
			ServerName:  "example.org",
			Fingerprint: "chrome",
			PublicKey:   "pub123",
			ShortID:     "sid123",
		},
		{
			Tag:      "Trojan-WS",
			Type:     model.ProtoTrojan,
			Server:   "tr.example.org",
			Port:     443,
			Password: "trojanpass",
			Security: model.SecurityTLS,
			Network:  model.TransportWS,
			Path:     "/ws",
			Host:     "tr.example.org",
		},
		{
			Tag:      "Hy2-Test",
			Type:     model.ProtoHysteria2,
			Server:   "hy2.example.org",
			Port:     8443,
			Password: "hy2pass",
			Security: model.SecurityTLS,
			UpMbps:   50,
			DownMbps: 100,
		},
		{
			Tag:         "VMess-WS",
			Type:        model.ProtoVMess,
			Server:      "vmess.example.org",
			Port:        443,
			UUID:        "22222222-3333-4444-5555-666666666666",
			Security:    model.SecurityTLS,
			Network:     model.TransportWS,
			Path:        "/vpath",
			ServerName:  "vmess.example.org",
			Fingerprint: "chrome",
		},
		{
			Tag:        "WG-Node",
			Type:       model.ProtoWireGuard,
			Server:     "1.2.3.4",
			Port:       51820,
			PeerPubKey: "fakePub",
			PrivateKey: "fakePriv",
			LocalAddr:  []string{"10.0.0.2/32"},
			MTU:        1420,
		},
	}
}

func TestSingboxRoundTrip(t *testing.T) {
	orig := sampleNodes()

	// Emit to sing-box JSON
	data, err := emitter.ToSingboxJSON(orig)
	if err != nil {
		t.Fatalf("ToSingboxJSON failed: %v", err)
	}

	// Parse back
	parsed, err := parser.ParseSingbox(data)
	if err != nil {
		t.Fatalf("ParseSingbox failed: %v", err)
	}

	if len(parsed) != len(orig) {
		t.Fatalf("expected %d nodes, got %d", len(orig), len(parsed))
	}

	if parsed[0].Tag != "SS-Test" || parsed[0].Method != "aes-128-gcm" {
		t.Fatalf("SS mismatch")
	}
	if parsed[1].Tag != "VLESS-Reality" || parsed[1].PublicKey != "pub123" {
		t.Fatalf("VLESS mismatch")
	}
	if parsed[2].Tag != "Trojan-WS" || parsed[2].Path != "/ws" {
		t.Fatalf("Trojan mismatch")
	}
	if parsed[3].Tag != "Hy2-Test" || parsed[3].UpMbps != 50 {
		t.Fatalf("Hy2 mismatch")
	}
}

func TestClashRoundTrip(t *testing.T) {
	orig := sampleNodes()

	// Emit to Clash YAML
	data, err := emitter.ToClashYAML(orig)
	if err != nil {
		t.Fatalf("ToClashYAML failed: %v", err)
	}

	// Parse back
	parsed, err := parser.ParseClash(data)
	if err != nil {
		t.Fatalf("ParseClash failed: %v", err)
	}

	if len(parsed) != len(orig) {
		t.Fatalf("expected %d nodes, got %d", len(orig), len(parsed))
	}

	if parsed[0].Tag != "SS-Test" || parsed[0].Method != "aes-128-gcm" {
		t.Fatalf("SS mismatch")
	}
	if parsed[1].Tag != "VLESS-Reality" || parsed[1].PublicKey != "pub123" {
		t.Fatalf("VLESS mismatch")
	}
	if parsed[2].Tag != "Trojan-WS" || parsed[2].Path != "/ws" {
		t.Fatalf("Trojan mismatch")
	}
	if parsed[3].Tag != "Hy2-Test" || parsed[3].UpMbps != 50 {
		t.Fatalf("Hy2 mismatch")
	}
}

func TestBase64SubscriptionAndFetcher(t *testing.T) {
	orig := sampleNodes()
	var uris string
	for _, n := range orig {
		u, err := emitter.ToURI(n)
		if err != nil {
			t.Fatalf("ToURI failed: %v", err)
		}
		uris += u + "\n"
	}

	b64Sub := base64.StdEncoding.EncodeToString([]byte(uris))

	// Test ParseSubscription with Base64 payload
	nodes, err := parser.ParseSubscription([]byte(b64Sub))
	if err != nil {
		t.Fatalf("ParseSubscription base64 failed: %v", err)
	}
	if len(nodes) != len(orig) {
		t.Fatalf("expected %d nodes, got %d", len(orig), len(nodes))
	}

	// Test Fetcher with HTTP mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(b64Sub))
	}))
	defer ts.Close()

	f := fetcher.New(5 * time.Second)
	fetchedNodes, err := f.FetchSubscription(t.Context(), ts.URL, "custom-ua")
	if err != nil {
		t.Fatalf("FetchSubscription failed: %v", err)
	}

	if len(fetchedNodes) != len(orig) {
		t.Fatalf("expected %d fetched nodes, got %d", len(orig), len(fetchedNodes))
	}
}
