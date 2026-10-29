package emitter

import (
	"encoding/json"
	"fmt"

	"github.com/arisvia/subrefine/internal/core/model"
	"gopkg.in/yaml.v3"
)

// ClashFullTemplate is the mihomo counterpart of SingboxTemplate: an
// operator-authored full config (ports/tun/proxy-groups/rules) with node
// injection markers. mihomo already accepts a full YAML over a subscription
// URL, so this makes subrefine the single source for both client families.
//
// Template markers (see fullmarkers.go):
//
//	proxies entry     {- name: "@subrefine" }   injection point, replaced in
//	                                          place by the node proxies
//	proxy-groups      proxies: ["@subrefine"]   expands to every delivered
//	                                            proxy name
//	regex group       proxies: ["@subrefine:(?i)(HK|港)"]
//
// Unlike sing-box, WireGuard nodes are kept (mihomo has a wireguard proxy
// type), so clash full delivery carries the complete node set.
type ClashFullTemplate struct {
	raw     []byte
	regexes markerRegexes
}

// LoadClashFullTemplate parses and validates a clash template.
func LoadClashFullTemplate(data []byte) (*ClashFullTemplate, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("clash template is not valid yaml: %w", err)
	}
	list, _ := doc["proxies"].([]any)
	if list == nil {
		return nil, fmt.Errorf(`clash template needs a "proxies" list to inject into`)
	}

	injections := 0
	regexes := markerRegexes{}
	for _, p := range list {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == markerPrefix {
			injections++
		}
	}
	groups, _ := doc["proxy-groups"].([]any)
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		refs, _ := gm["proxies"].([]any)
		for _, ref := range refs {
			if s, ok := ref.(string); ok {
				if err := regexes.collectMarker(s); err != nil {
					return nil, err
				}
			}
		}
	}
	if injections != 1 {
		return nil, fmt.Errorf(`clash template needs exactly one {name: "%s"} injection proxy, found %d`, markerPrefix, injections)
	}

	return &ClashFullTemplate{raw: data, regexes: regexes}, nil
}

// Render splices the node proxies into a fresh copy of the template.
func (t *ClashFullTemplate) Render(nodes []*model.ProxyNode) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(t.raw, &doc); err != nil {
		return nil, fmt.Errorf("clash template re-parse: %w", err)
	}

	var (
		nodeProxies []any
		nodeNames   []string
	)
	for _, n := range nodes {
		p, err := ToClashProxy(n)
		if err != nil || p == nil {
			continue
		}
		nodeProxies = append(nodeProxies, p)
		nodeNames = append(nodeNames, n.Tag)
	}
	if len(nodeProxies) == 0 {
		return nil, ErrNoDeliverableNodes
	}

	list, _ := doc["proxies"].([]any)
	rendered := make([]any, 0, len(list)+len(nodeProxies))
	for _, p := range list {
		m, ok := p.(map[string]any)
		if !ok || m["name"] != markerPrefix {
			rendered = append(rendered, p)
			continue
		}
		rendered = append(rendered, nodeProxies...)
	}
	doc["proxies"] = rendered

	groups, _ := doc["proxy-groups"].([]any)
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		refs, isList := gm["proxies"].([]any)
		if !isList {
			continue
		}
		expanded, err := expandMarkerList(refs, nodeNames, t.regexes)
		if err != nil {
			return nil, err
		}
		gm["proxies"] = expanded
	}
	doc["proxy-groups"] = groups

	// Same leak guard as sing-box: a marker outside the two expansion sites
	// would reach the client as a dangling group member.
	rest, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("clash template render: %w", err)
	}
	if hasMarker(rest) {
		return nil, fmt.Errorf("clash template contains %s markers outside proxies/proxy-groups lists", markerPrefix)
	}

	return yaml.Marshal(doc)
}
