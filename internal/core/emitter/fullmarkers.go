package emitter

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Shared marker machinery for full-config templates (sing-box and clash).
// A marker is any string starting with "@subrefine":
//
//	"@subrefine"            all delivered node names
//	"@subrefine:<regex>"    node names matching regex (substring search);
//	                        zero matches falls back to all nodes so the
//	                        referencing group stays loadable

const markerPrefix = "@subrefine"

var markerPrefixBytes = []byte(markerPrefix)

// ErrNoDeliverableNodes is returned by full-config renders when the node set
// contributes nothing injectable: a full config whose groups reference zero
// members is rejected by both clients, so callers must fall back to the
// fragment form (which handles empties explicitly) instead.
var ErrNoDeliverableNodes = errors.New("no deliverable nodes for full config")

type markerRegexes map[string]*regexp.Regexp

// collectMarker compiles (and caches) the regex behind one marker string.
func (m markerRegexes) collectMarker(s string) error {
	if s == markerPrefix || !strings.HasPrefix(s, markerPrefix+":") {
		return nil
	}
	if _, seen := m[s]; seen {
		return nil
	}
	re, err := regexp.Compile(strings.TrimPrefix(s, markerPrefix+":"))
	if err != nil {
		return fmt.Errorf("template marker %q: %w", s, err)
	}
	m[s] = re
	return nil
}

// expandMarkerList rewrites a group's name list, replacing marker entries
// with concrete node names while preserving order and literal entries.
func expandMarkerList(refs []any, nodeNames []string, rx markerRegexes) ([]any, error) {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		s, ok := ref.(string)
		if !ok || !strings.HasPrefix(s, markerPrefix) {
			out = append(out, ref)
			continue
		}
		if s == markerPrefix {
			for _, name := range nodeNames {
				out = append(out, name)
			}
			continue
		}
		re, found := rx[s]
		if !found {
			return nil, fmt.Errorf("uncompiled marker %q in template", s)
		}
		matched := 0
		for _, name := range nodeNames {
			if re.MatchString(name) {
				out = append(out, name)
				matched++
			}
		}
		if matched == 0 {
			// When a region filter matches zero nodes, sing-box rejects an
			// empty outbounds list (FATAL: missing tags). Instead of dumping
			// all unrelated nodes into the group (defeating grouping), fall
			// back to "Direct" (compatible mode). Avoid duplicate if Direct
			// is already an explicit member.
			alreadyDirect := false
			for _, o := range out {
				if o == "Direct" {
					alreadyDirect = true
					break
				}
			}
			if !alreadyDirect {
				out = append(out, "Direct")
			}
		}
	}
	// Deduplicate preserving order (e.g. static "Direct" after a fallback "Direct").
	seen := make(map[any]bool, len(out))
	deduped := make([]any, 0, len(out))
	for _, item := range out {
		if !seen[item] {
			seen[item] = true
			deduped = append(deduped, item)
		}
	}
	return deduped, nil
}

// hasMarker reports whether the serialized document still contains a marker
// (which would mean it leaked into a place no list expansion covers).
func hasMarker(data []byte) bool {
	return bytes.Contains(data, markerPrefixBytes)
}
