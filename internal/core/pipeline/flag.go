package pipeline

import (
	"regexp"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

// A country flag is a pair of Regional Indicator Symbols (U+1F1E6..U+1F1FF).
var flagPairRE = regexp.MustCompile(`[\x{1F1E6}-\x{1F1FF}]{2}`)

// normalizeFlagKey lower-cases and strips separators/digits so a dictionary
// key and a tag fragment compare on their letters and CJK runes only.
func normalizeFlagKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '\u4e00' && r <= '\u9fff':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// detectISO2 returns the region code for the first (longest) dictionary key
// present in the tag, or "" when nothing recognizable is found.
func detectISO2(tag string) string {
	compact := normalizeFlagKey(tag)
	for _, e := range flagLong {
		if strings.Contains(compact, e.key) {
			return e.code
		}
	}
	// Short abbreviations only fire as standalone ALL-CAPS tokens, so "US"
	// matches "US-01"/"LA 03" but the lowercase "no"/"in"/"us" inside ordinary
	// English words never triggers a flag.
	for _, w := range wordRE.FindAllString(tag, -1) {
		if w != strings.ToUpper(w) {
			continue
		}
		if code, ok := flagAbbr[strings.ToLower(w)]; ok {
			return code
		}
	}
	return ""
}

var wordRE = regexp.MustCompile(`[A-Za-z]{2,3}`)

// flagEmoji renders an ISO 3166-1 alpha-2 code as a flag; "" for anything else.
func flagEmoji(code string) string {
	code = strings.ToUpper(code)
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	return string([]rune{0x1F1E6 + rune(code[0]-'A'), 0x1F1E6 + rune(code[1]-'A')})
}

const (
	FlagModeAdd       = "add"
	FlagModeRemove    = "remove"
	FlagModeNormalize = "normalize"
)

// Flag adds or removes country-flag emoji on node tags.
//   - add: prepend the detected flag, but only when the tag has none yet.
//   - remove: strip every flag pair.
//   - normalize: strip first, then re-detect and prepend (idempotent refresh).
//
// Nodes whose tag matches no dictionary key are left untouched; cover those by
// adding a rename step before this one.
func Flag(nodes []*model.ProxyNode, mode string) []*model.ProxyNode {
	out := make([]*model.ProxyNode, len(nodes))
	for i, n := range nodes {
		c := n.Clone()
		c.Tag = applyFlag(c.Tag, mode)
		out[i] = c
	}
	return out
}

func applyFlag(tag, mode string) string {
	switch mode {
	case FlagModeRemove:
		return trimSpaces(flagPairRE.ReplaceAllString(tag, ""))
	case FlagModeNormalize:
		return prependFlag(trimSpaces(flagPairRE.ReplaceAllString(tag, "")))
	default: // add
		if flagPairRE.MatchString(tag) {
			return tag
		}
		return prependFlag(tag)
	}
}

func prependFlag(tag string) string {
	flag := flagEmoji(detectISO2(tag))
	if flag == "" {
		return tag
	}
	return flag + " " + tag
}

var multiSpaceRE = regexp.MustCompile(`\s{2,}`)

func trimSpaces(s string) string {
	return strings.TrimSpace(multiSpaceRE.ReplaceAllString(s, " "))
}
