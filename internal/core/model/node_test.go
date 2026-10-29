package model_test

import (
	"testing"

	"github.com/arisvia/subrefine/internal/core/model"
)

func TestIdentityStableAcrossRename(t *testing.T) {
	n := &model.ProxyNode{
		Tag: "HK-01", Type: model.ProtoShadowsocks,
		Server: "1.1.1.1", Port: 8388, Method: "aes-128-gcm", Password: "p",
	}
	id := n.Identity()

	// Cosmetic pipeline edits must not move the identity: per-key disable
	// lists are written once and survive upstream renames and decoration.
	n.Tag = "🇭 Hong Kong 01"
	n.Fingerprint = "chrome"
	n.Detour = "WARP"
	if n.Identity() != id {
		t.Fatalf("identity moved after rename/decorate: %s -> %s", id, n.Identity())
	}

	// A real connection change must move it.
	m := n.Clone()
	m.Port = 443
	if m.Identity() == id {
		t.Fatal("identity survived a port change")
	}
	w := n.Clone()
	w.Server = "9.9.9.9"
	if w.Identity() == id {
		t.Fatal("identity survived a server change")
	}
}

func TestFilterDisabled(t *testing.T) {
	a := &model.ProxyNode{Tag: "A", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 1, Method: "m", Password: "p"}
	b := &model.ProxyNode{Tag: "B", Type: model.ProtoTrojan, Server: "2.2.2.2", Port: 2, Password: "p"}
	c := &model.ProxyNode{Tag: "C", Type: model.ProtoVMess, Server: "3.3.3.3", Port: 3, UUID: "u"}
	nodes := []*model.ProxyNode{a, b, c}

	if got := model.FilterDisabled(nodes, nil); &got[0] != &nodes[0] {
		t.Fatal("empty blacklist must return the shared cached slice untouched")
	}

	out := model.FilterDisabled(nodes, []string{b.Identity()})
	if len(out) != 2 || out[0] != a || out[1] != c {
		t.Fatalf("blacklist filter wrong: %v", tags(out))
	}
	if len(nodes) != 3 {
		t.Fatal("input slice mutated - the delivery cache is shared")
	}

	// Stale identities (node gone upstream) simply match nothing.
	if got := model.FilterDisabled(nodes, []string{"ffffffffffff"}); len(got) != 3 {
		t.Fatalf("stale identity must not drop live nodes, got %d", len(got))
	}
}

func tags(ns []*model.ProxyNode) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Tag
	}
	return out
}
