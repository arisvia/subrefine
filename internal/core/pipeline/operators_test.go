package pipeline_test

import (
	"testing"

	"github.com/arisvia/subrefine/internal/core/model"
	"github.com/arisvia/subrefine/internal/core/pipeline"
)

func nodes(specs ...[2]string) []*model.ProxyNode {
	out := make([]*model.ProxyNode, 0, len(specs))
	for i, s := range specs {
		out = append(out, &model.ProxyNode{
			Tag:    s[0],
			Type:   s[1],
			Server: "10.0.0." + string(rune('1'+i)),
			Port:   443,
		})
	}
	return out
}

func tags(list []*model.ProxyNode) []string {
	out := make([]string, len(list))
	for i, n := range list {
		out[i] = n.Tag
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterByProtocol(t *testing.T) {
	in := nodes([2]string{"A", model.ProtoShadowsocks}, [2]string{"B", model.ProtoVLESS}, [2]string{"C", model.ProtoTrojan})

	got := tags(pipeline.Filter(in, "", "", model.ProtoVLESS, model.ProtoTrojan))
	if !equal(got, []string{"B", "C"}) {
		t.Fatalf("protocol allow-list: got %v", got)
	}

	// No types supplied -> protocol agnostic, tag regex still applies.
	got = tags(pipeline.Filter(in, "A|C", ""))
	if !equal(got, []string{"A", "C"}) {
		t.Fatalf("tag include: got %v", got)
	}

	// Blank type entries must not silently drop everything.
	got = tags(pipeline.Filter(in, "", "", "", "  "))
	if !equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("blank types ignored: got %v", got)
	}

	if len(pipeline.Filter(in, "", "A", model.ProtoShadowsocks)) != 0 {
		t.Fatal("exclude + type intersection should drop all")
	}
}

func TestSort(t *testing.T) {
	in := nodes([2]string{"US-02", model.ProtoTrojan}, [2]string{"HK-01", model.ProtoShadowsocks}, [2]string{"JP-03", model.ProtoVLESS})

	if got := tags(pipeline.Sort(in, "tag", "asc")); !equal(got, []string{"HK-01", "JP-03", "US-02"}) {
		t.Fatalf("asc by tag: got %v", got)
	}
	if got := tags(pipeline.Sort(in, "tag", "desc")); !equal(got, []string{"US-02", "JP-03", "HK-01"}) {
		t.Fatalf("desc by tag: got %v", got)
	}
	// Default order is ascending when unspecified.
	if got := tags(pipeline.Sort(in, "", "")); !equal(got, []string{"HK-01", "JP-03", "US-02"}) {
		t.Fatalf("default order: got %v", got)
	}
	byProto := tags(pipeline.Sort(in, "protocol", "asc"))
	if !equal(byProto, []string{"HK-01", "US-02", "JP-03"}) { // shadowsocks < trojan < vless
		t.Fatalf("asc by protocol: got %v", byProto)
	}
	// Input slice order must not be mutated (callers keep their pipeline result).
	if got := tags(in); !equal(got, []string{"US-02", "HK-01", "JP-03"}) {
		t.Fatalf("Sort mutated input: got %v", got)
	}
}

func TestDeduplicateKeepsDistinctWireGuardPeers(t *testing.T) {
	// Every Cloudflare WARP peer terminates on the same endpoint and differs
	// only by key material.
	peer := func(pub, priv string) *model.ProxyNode {
		return &model.ProxyNode{
			Tag: "WARP " + pub, Type: model.ProtoWireGuard,
			Server: "162.159.192.1", Port: 2408,
			PeerPubKey: pub, PrivateKey: priv,
		}
	}
	a, b := peer("pubAAA", "privAAA"), peer("pubBBB", "privBBB")
	sameAsA := peer("pubAAA", "privAAA")
	sameAsA.Tag = "WARP pubAAA copy"

	got := tags(pipeline.Deduplicate([]*model.ProxyNode{a, b, sameAsA}))
	if !equal(got, []string{"WARP pubAAA", "WARP pubBBB"}) {
		t.Fatalf("distinct wg peers collapsed: %v", got)
	}

	// Pre-shared key and obfs password must also count as identity.
	p1 := &model.ProxyNode{Tag: "p1", Type: model.ProtoHysteria2, Server: "1.1.1.1", Port: 443, Password: "x", PreSharedKey: "psk1"}
	p2 := &model.ProxyNode{Tag: "p2", Type: model.ProtoHysteria2, Server: "1.1.1.1", Port: 443, Password: "x", PreSharedKey: "psk2"}
	if len(pipeline.Deduplicate([]*model.ProxyNode{p1, p2})) != 2 {
		t.Fatal("psk difference ignored")
	}
}

func TestFilterProtocolAliases(t *testing.T) {
	in := nodes([2]string{"A", model.ProtoShadowsocks}, [2]string{"B", model.ProtoVLESS}, [2]string{"C", model.ProtoWireGuard})

	// Shorthand spellings must resolve to the canonical protocols.
	if got := tags(pipeline_FilterAlias(in, "ss", "wg")); !equal(got, []string{"A", "C"}) {
		t.Fatalf("aliases not normalized: %v", got)
	}
	// Mixed case and padding are tolerated.
	if got := tags(pipeline_FilterAlias(in, " Hy2 ", "VLESS")); !equal(got, []string{"B"}) {
		t.Fatalf("case/space handling: %v", got)
	}
	// An unknown name must not silently empty the subscription.
	if got := tags(pipeline_FilterAlias(in, "shadowquic")); !equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("unknown protocol dropped everything: %v", got)
	}
	// Unknown + known still filters by the known one.
	if got := tags(pipeline_FilterAlias(in, "nope", "ss")); !equal(got, []string{"A"}) {
		t.Fatalf("mixed alias list: %v", got)
	}
}

func pipeline_FilterAlias(in []*model.ProxyNode, types ...string) []*model.ProxyNode {
	return pipeline.Filter(in, "", "", types...)
}

func TestDeduplicateKeepsFirstTagAndDistinctProtocols(t *testing.T) {
	a := &model.ProxyNode{Tag: "first", Type: model.ProtoShadowsocks, Server: "1.1.1.1", Port: 8388, Method: "aes-256-gcm", Password: "p"}
	dup := a.Clone()
	dup.Tag = "second"
	sameCredsOtherProto := &model.ProxyNode{Tag: "trojan-dup", Type: model.ProtoTrojan, Server: "1.1.1.1", Port: 8388, Password: "p"}

	got := pipeline.Deduplicate([]*model.ProxyNode{a, dup, sameCredsOtherProto})
	if tags(got) == nil || !equal(tags(got), []string{"first", "trojan-dup"}) {
		t.Fatalf("dedup result: %v", tags(got))
	}
}
