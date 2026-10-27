package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

// RegexRename applies regex pattern replacement to node tags.
func RegexRename(nodes []*model.ProxyNode, pattern, repl string) []*model.ProxyNode {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nodes
	}
	out := make([]*model.ProxyNode, len(nodes))
	for i, n := range nodes {
		cpy := n.Clone()
		cpy.Tag = strings.TrimSpace(re.ReplaceAllString(cpy.Tag, repl))
		out[i] = cpy
	}
	return out
}

// Filter retains nodes that match the include keyword/regex, or drops those that match exclude.
func Filter(nodes []*model.ProxyNode, includePattern, excludePattern string) []*model.ProxyNode {
	var incRe, excRe *regexp.Regexp
	if includePattern != "" {
		incRe, _ = regexp.Compile(includePattern)
	}
	if excludePattern != "" {
		excRe, _ = regexp.Compile(excludePattern)
	}

	out := make([]*model.ProxyNode, 0, len(nodes))
	for _, n := range nodes {
		if incRe != nil && !incRe.MatchString(n.Tag) {
			continue
		}
		if excRe != nil && excRe.MatchString(n.Tag) {
			continue
		}
		out = append(out, n.Clone())
	}
	return out
}

// Deduplicate removes duplicate nodes based on server + port + protocol + credentials.
func Deduplicate(nodes []*model.ProxyNode) []*model.ProxyNode {
	seen := make(map[string]bool, len(nodes))
	out := make([]*model.ProxyNode, 0, len(nodes))

	for _, n := range nodes {
		sig := fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s",
			n.Type, n.Server, n.Port, n.UUID, n.Password, n.Method, n.Path)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, n.Clone())
	}
	return out
}

// Decorator updates node properties across a slice of nodes.
type DecoratorOptions struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	Detour      string `json:"detour,omitempty"`
	Insecure    *bool  `json:"insecure,omitempty"`
}

func Decorate(nodes []*model.ProxyNode, opts DecoratorOptions) []*model.ProxyNode {
	out := make([]*model.ProxyNode, len(nodes))
	for i, n := range nodes {
		cpy := n.Clone()
		if opts.Fingerprint != "" {
			cpy.Fingerprint = opts.Fingerprint
		}
		if opts.Detour != "" {
			cpy.Detour = opts.Detour
		}
		if opts.Insecure != nil {
			cpy.Insecure = *opts.Insecure
		}
		out[i] = cpy
	}
	return out
}
