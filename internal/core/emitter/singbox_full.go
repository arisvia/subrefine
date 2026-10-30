package emitter

import (
	"encoding/json"
	"fmt"

	"github.com/arisvia/subrefine/internal/core/model"
)

// Official sing-box has no subscription/provider concept: a client pointed at
// a URL must receive a complete, loadable config. The full-config target
// takes an operator-authored template (log/dns/route/inbounds skeleton) and
// splices the subscription's nodes into it, producing pure official schema.
//
// Template markers (see fullmarkers.go):
//
//	outbounds entry   {"type":"direct","tag":"@subrefine"}  injection point,
//	                  replaced in place by the node outbounds
//	group reference   "outbounds": ["@subrefine"]           expands to every
//	                  delivered node tag
//	regex group       "outbounds": ["@subrefine:(?i)(HK|港)"] expands to the
//	                  node tags matching the regex (substring search, like the
//	                  fork clients' urltest "include")

// SingboxTemplate is a parsed, validated full-config template. The raw bytes
// are re-decoded per render so a template is never mutated by delivery.
type SingboxTemplate struct {
	raw     []byte
	regexes markerRegexes
}

// LoadSingboxTemplate parses and validates a template: it must be a JSON
// object with an outbounds array containing exactly one injection point.
func LoadSingboxTemplate(data []byte) (*SingboxTemplate, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("sing-box template is not valid json: %w", err)
	}
	list, _ := doc["outbounds"].([]any)
	if list == nil {
		return nil, fmt.Errorf(`sing-box template needs an "outbounds" array to inject into`)
	}

	injections := 0
	regexes := markerRegexes{}
	for _, ob := range list {
		m, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if m["tag"] == markerPrefix {
			injections++
		}
		if refs, ok := m["outbounds"].([]any); ok {
			for _, ref := range refs {
				if s, ok := ref.(string); ok {
					if err := regexes.collectMarker(s); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if injections != 1 {
		return nil, fmt.Errorf(`sing-box template needs exactly one {"type":"direct","tag":"%s"} injection outbound, found %d`, markerPrefix, injections)
	}

	return &SingboxTemplate{raw: data, regexes: regexes}, nil
}

// InboundOptions reports which managed inbound kinds (mixed/tun/tproxy) this
// template actually declares, so the console only offers switches that mean
// something for this template.
func (t *SingboxTemplate) InboundOptions() []string {
	var doc map[string]any
	if err := json.Unmarshal(t.raw, &doc); err != nil {
		return nil
	}
	var found []string
	if ins, ok := doc["inbounds"].([]any); ok {
		for _, in := range ins {
			if im, ok := in.(map[string]any); ok {
				if tp, _ := im["type"].(string); managedInbounds[tp] {
					found = append(found, tp)
				}
			}
		}
	}
	return sortOptions(found)
}

// Render splices the deliverable node outbounds into a fresh copy of the
// template, keeps only the inbounds the selected options enable, and applies
// the IPv6 posture. Nodes the sing-box target cannot express (WireGuard, see
// SingboxSkipped) fail ToSingboxOutbound and are skipped, matching the
// fragment emitter.
func (t *SingboxTemplate) Render(nodes []*model.ProxyNode, opts FullOptions) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(t.raw, &doc); err != nil {
		return nil, fmt.Errorf("sing-box template re-parse: %w", err)
	}

	var (
		nodeOutbounds []any
		nodeTags      []string
	)
	for _, n := range nodes {
		ob, err := ToSingboxOutbound(n)
		if err != nil || ob == nil {
			continue
		}
		nodeOutbounds = append(nodeOutbounds, ob)
		nodeTags = append(nodeTags, n.Tag)
	}
	if len(nodeOutbounds) == 0 {
		return nil, ErrNoDeliverableNodes
	}

	list, _ := doc["outbounds"].([]any)
	rendered := make([]any, 0, len(list)+len(nodeOutbounds))
	for _, ob := range list {
		m, ok := ob.(map[string]any)
		if !ok {
			rendered = append(rendered, ob)
			continue
		}
		if m["tag"] == markerPrefix {
			rendered = append(rendered, nodeOutbounds...)
			continue
		}
		if refs, ok := m["outbounds"].([]any); ok {
			expanded, err := expandMarkerList(refs, nodeTags, t.regexes)
			if err != nil {
				return nil, err
			}
			m["outbounds"] = expanded
		}
		rendered = append(rendered, m)
	}
	doc["outbounds"] = rendered

	// Keep only the inbounds the selected options enable; unmanaged (custom)
	// inbounds always survive.
	if ins, ok := doc["inbounds"].([]any); ok {
		kept := make([]any, 0, len(ins))
		for _, in := range ins {
			im, ok := in.(map[string]any)
			if !ok {
				kept = append(kept, in)
				continue
			}
			tp, _ := im["type"].(string)
			if keepInbound(tp, opts.Inbounds) {
				kept = append(kept, in)
			}
		}
		doc["inbounds"] = kept
	}
	applySingboxIPv6(doc, opts.IPv6)

	// A marker may only legitimately appear as an injection point or a group
	// reference; anything still present means the template referenced nodes
	// where sing-box cannot carry a list. Fail loudly, never ship a
	// placeholder.
	rest, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("sing-box template render: %w", err)
	}
	if hasMarker(rest) {
		return nil, fmt.Errorf("sing-box template contains %s markers outside outbounds group lists", markerPrefix)
	}

	return json.MarshalIndent(doc, "", "  ")
}
