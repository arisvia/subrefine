package emitter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/model"
)

// sing-box 1.13.0 removed the wireguard outbound, and its endpoint
// replacement is a startup tunnel - not a selectable per-node proxy. The
// sing-box target must therefore leave WG out of the config AND announce
// the omission, because a silently vanished node is the failure mode this
// project already outlawed in the pipeline.
func TestSingboxSkipsWireGuardVisibly(t *testing.T) {
	nodes := []*model.ProxyNode{
		{
			Tag: "WG-HK", Type: model.ProtoWireGuard,
			Server: "203.0.113.9", Port: 51820,
			PrivateKey: "wA9E3J5UwM3N0mU2Q2nD1K7pS8vT4xR6bY9cE0fGhIk=",
			PeerPubKey: "hB3nQ7sV0xZ2cD5fG8jK1mN4pR7tW9yA2eI5oU8wY0A=",
			LocalAddr:  []string{"10.66.66.2"},
		},
		{
			Tag: "SS-JP", Type: model.ProtoShadowsocks,
			Server: "1.2.3.4", Port: 8388, Method: "aes-128-gcm", Password: "p",
		},
	}

	skipped := emitter.SingboxSkipped(nodes)
	if len(skipped) != 1 || skipped[0] != "WG-HK" {
		t.Fatalf("SingboxSkipped = %v, want [WG-HK]", skipped)
	}

	data, err := emitter.ToSingboxJSON(nodes)
	if err != nil {
		t.Fatalf("ToSingboxJSON: %v", err)
	}
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
		Endpoints []map[string]any `json:"endpoints"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v (%s)", err, data)
	}
	if len(doc.Outbounds) != 1 || doc.Outbounds[0]["tag"] != "SS-JP" {
		t.Fatalf("outbounds = %v, want only SS-JP", doc.Outbounds)
	}
	if len(doc.Endpoints) != 0 {
		t.Fatalf("endpoints = %v, want none", doc.Endpoints)
	}
}

// Clash targets still carry WG; the skip is sing-box-specific.
func TestClashKeepsWireGuard(t *testing.T) {
	data, err := emitter.ToClashYAML([]*model.ProxyNode{{
		Tag: "WG-HK", Type: model.ProtoWireGuard,
		Server: "203.0.113.9", Port: 51820,
		PrivateKey: "wA9E3J5UwM3N0mU2Q2nD1K7pS8vT4xR6bY9cE0fGhIk=",
		PeerPubKey: "hB3nQ7sV0xZ2cD5fG8jK1mN4pR7tW9yA2eI5oU8wY0A=",
		LocalAddr:  []string{"10.66.66.2/32"},
	}})
	if err != nil {
		t.Fatalf("ToClashYAML: %v", err)
	}
	s := string(data)
	for _, want := range []string{"type: wireguard", "private-key:", "public-key:"} {
		if !strings.Contains(s, want) {
			t.Fatalf("clash output missing %q:\n%s", want, s)
		}
	}
}

// mihomo refuses to load a config whose url-test group has no proxies, so a
// fully filtered subscription must fall back to a bare DIRECT ruleset.
func TestClashZeroNodesStaysLoadable(t *testing.T) {
	data, err := emitter.ToClashYAML([]*model.ProxyNode{})
	if err != nil {
		t.Fatalf("ToClashYAML: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "MATCH,DIRECT") || strings.Contains(s, "url-test") {
		t.Fatalf("zero-node clash output must be a bare DIRECT config:\n%s", s)
	}
}
