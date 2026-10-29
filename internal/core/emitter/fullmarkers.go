package emitter

import (
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
			// An empty selector/url-test list makes either client reject the
			// whole config; a region group with no current members degrades
			// to "all nodes" rather than breaking delivery.
			for _, name := range nodeNames {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

// hasMarker reports whether the serialized document still contains a marker
// (which would mean it leaked into a place no list expansion covers).
func hasMarker(jsonBytes []byte) bool {
	return strings.Contains(string(jsonBytes), markerPrefix)
}
