package pipeline

import (
	"regexp"
	"sort"
	"strconv"
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

// Filter retains nodes whose tag matches includePattern but not excludePattern,
// and whose protocol is in types (empty, or nothing recognizable, = all protocols).
// Protocol names accept the usual aliases (ss, hy2, wg, ...); unknown entries are
// dropped rather than treated as a constraint, so a typo can never deliver zero nodes.
func Filter(nodes []*model.ProxyNode, includePattern, excludePattern string, types ...string) []*model.ProxyNode {
	var incRe, excRe *regexp.Regexp
	if includePattern != "" {
		incRe, _ = regexp.Compile(includePattern)
	}
	if excludePattern != "" {
		excRe, _ = regexp.Compile(excludePattern)
	}

	var allowed map[string]bool
	for _, t := range types {
		canon, ok := normalizeProtocol(t)
		if !ok {
			continue
		}
		if allowed == nil {
			allowed = make(map[string]bool, len(types))
		}
		allowed[canon] = true
	}

	out := make([]*model.ProxyNode, 0, len(nodes))
	for _, n := range nodes {
		if allowed != nil && !allowed[n.Type] {
			continue
		}
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

// protoAliases maps the spellings operators and users actually type onto the
// canonical model.Proto* values.
var protoAliases = map[string]string{
	"ss":          model.ProtoShadowsocks,
	"shadowsocks": model.ProtoShadowsocks,
	"vmess":       model.ProtoVMess,
	"vless":       model.ProtoVLESS,
	"tr":          model.ProtoTrojan,
	"trojan":      model.ProtoTrojan,
	"hy":          model.ProtoHysteria2,
	"hy2":         model.ProtoHysteria2,
	"hysteria":    model.ProtoHysteria2,
	"hysteria2":   model.ProtoHysteria2,
	"wg":          model.ProtoWireGuard,
	"wireguard":   model.ProtoWireGuard,
}

func normalizeProtocol(s string) (string, bool) {
	canon, ok := protoAliases[strings.TrimSpace(strings.ToLower(s))]
	return canon, ok
}

// Sort orders nodes in place by tag (default) or protocol, ascending or descending.
func Sort(nodes []*model.ProxyNode, by, order string) []*model.ProxyNode {
	out := append([]*model.ProxyNode(nil), nodes...)
	key := func(n *model.ProxyNode) string {
		switch by {
		case "protocol", "type":
			return n.Type + "|" + n.Tag
		default:
			return n.Tag
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if strings.EqualFold(order, "desc") {
			return key(out[i]) > key(out[j])
		}
		return key(out[i]) < key(out[j])
	})
	return out
}

// Deduplicate removes nodes that are the same endpoint *and* the same identity.
// Key material is part of the signature: many Cloudflare WARP peers share
// 162.159.192.1:2408 and differ only by keys, so omitting them would drop every
// peer but the first.
func Deduplicate(nodes []*model.ProxyNode) []*model.ProxyNode {
	seen := make(map[string]bool, len(nodes))
	out := make([]*model.ProxyNode, 0, len(nodes))

	for _, n := range nodes {
		sig := strings.Join([]string{
			n.Type, n.Server, strconv.Itoa(n.Port),
			n.UUID, n.Password, n.Method, n.Path,
			n.PrivateKey, n.PeerPubKey, n.PreSharedKey, n.ObfsPass,
		}, "|")
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
