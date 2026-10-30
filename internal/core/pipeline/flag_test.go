package pipeline

import (
	"testing"

	"github.com/arisvia/subrefine/internal/core/model"
)

func TestFlagDetect(t *testing.T) {
	cases := []struct {
		tag  string
		want string // expected ISO2 from detectISO2, "" = none
	}{
		{"UnitedStates1", "US"},
		{"🇺 UnitedStates2", "US"},
		{"Los Angeles 03", "US"},
		{"DE Germany9", "DE"},
		{"美国01", "US"},
		{"日本东京02", "JP"},
		{"America West", "US"},
		{"Singapore 01", "SG"},
		{"US-01", "US"},
		{"JP Tokyo", "JP"},
		{"LA 03", "US"},
		{"HK-01", "HK"},
		// negatives: no recognizable country, must not fire on word chunks
		{"because-node", ""},
		{"thinking 01", ""},
		{"Server No.1", ""},
		{"premium line", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := detectISO2(c.tag); got != c.want {
			t.Errorf("detectISO2(%q) = %q, want %q", c.tag, got, c.want)
		}
	}
}

func TestFlagEmoji(t *testing.T) {
	if got := flagEmoji("US"); got != "🇺🇸" {
		t.Errorf("flagEmoji(US) = %q", got)
	}
	if got := flagEmoji("us"); got != "🇺🇸" {
		t.Errorf("flagEmoji(us) = %q", got)
	}
	if got := flagEmoji("JP"); got != "🇯🇵" {
		t.Errorf("flagEmoji(JP) = %q", got)
	}
	if got := flagEmoji("U"); got != "" {
		t.Errorf("flagEmoji(U) = %q, want empty", got)
	}
	if got := flagEmoji("1A"); got != "" {
		t.Errorf("flagEmoji(1A) = %q, want empty", got)
	}
}

func TestFlagModes(t *testing.T) {
	nodes := func(tags ...string) []*model.ProxyNode {
		out := make([]*model.ProxyNode, len(tags))
		for i, tg := range tags {
			out[i] = &model.ProxyNode{Tag: tg, Type: model.ProtoVLESS}
		}
		return out
	}
	tagAt := func(ns []*model.ProxyNode, i int) string { return ns[i].Tag }

	// add: prepends flag when none present; idempotent when already flagged.
	added := Flag(nodes("UnitedStates1", "🇯🇵 Japan2"), FlagModeAdd)
	if tagAt(added, 0) != "🇺🇸 UnitedStates1" {
		t.Errorf("add new = %q", tagAt(added, 0))
	}
	if tagAt(added, 1) != "🇯🇵 Japan2" {
		t.Errorf("add existing not idempotent = %q", tagAt(added, 1))
	}

	// remove: strips flag pairs, leaves the rest.
	removed := Flag(nodes("🇺🇸 UnitedStates1", "No Flag Here"), FlagModeRemove)
	if tagAt(removed, 0) != "UnitedStates1" {
		t.Errorf("remove = %q", tagAt(removed, 0))
	}
	if tagAt(removed, 1) != "No Flag Here" {
		t.Errorf("remove untouched = %q", tagAt(removed, 1))
	}

	// normalize: strips existing then re-detects (refreshes wrong/missing flag).
	norm := Flag(nodes("🇩🇪 Germany9", "🇺🇸 Tokyo01"), FlagModeNormalize)
	if tagAt(norm, 0) != "🇩🇪 Germany9" {
		t.Errorf("normalize correct = %q", tagAt(norm, 0))
	}
	// Tokyo -> JP even though a stale US flag was present.
	if tagAt(norm, 1) != "🇯🇵 Tokyo01" {
		t.Errorf("normalize refresh = %q", tagAt(norm, 1))
	}

	// input must not be mutated (shared cache safety).
	src := nodes("UnitedStates1")
	Flag(src, FlagModeAdd)
	if src[0].Tag != "UnitedStates1" {
		t.Errorf("Flag mutated input: %q", src[0].Tag)
	}
}
