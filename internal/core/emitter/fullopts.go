package emitter

import "sort"

// FamilyOptions describes what the console may offer for one client family:
// whether a full-config template is loaded, and which inbound kinds it
// declares. Lives in emitter so the api interface stays free of a service
// dependency (api defines the registry contract, service implements it).
type FamilyOptions struct {
	Available bool     `json:"available"`
	Inbounds  []string `json:"inbounds"`
}

// FullOptions are the per-delivery switches that turn a generic template into
// a concrete config. They are baked into the delivery URL, never stored on the
// subscription, so one subscription can serve any combination simultaneously.
type FullOptions struct {
	// Inbounds is the selected subset of inbound kinds to keep. Recognised
	// values: "mixed", "tun", "tproxy". Inbounds whose kind is not listed are
	// dropped; any other (custom) inbound is always kept.
	Inbounds []string
	// IPv6, when non-nil, forces the config's IPv6 posture: true => prefer
	// IPv6, false => IPv4-only. nil leaves the template's own setting intact.
	IPv6 *bool
}

// managedInbounds are the inbound kinds the option switch controls. Everything
// else in a template is operator-authored plumbing that always ships.
var managedInbounds = map[string]bool{"mixed": true, "tun": true, "tproxy": true}

func selected(set []string, want string) bool {
	for _, s := range set {
		if s == want {
			return true
		}
	}
	return false
}

// keepInbound reports whether a sing-box inbound (by its "type") survives the
// selected option set. Unmanaged types are always kept.
func keepInbound(inboundType string, sel []string) bool {
	if !managedInbounds[inboundType] {
		return true
	}
	return selected(sel, inboundType)
}

// clashKeyForOption maps a delivery option to the mihomo top-level keys it
// owns. Selecting an option keeps those keys; deselecting removes them.
var clashKeyForOption = map[string][]string{
	"mixed":  {"mixed-port", "port", "socks-port"},
	"tun":    {"tun"},
	"tproxy": {"tproxy-port", "redir-port"},
}

// applySingboxIPv6 sets the sing-box DNS strategy from the IPv6 switch.
func applySingboxIPv6(doc map[string]any, ipv6 *bool) {
	if ipv6 == nil {
		return
	}
	dns, _ := doc["dns"].(map[string]any)
	if dns == nil {
		dns = map[string]any{}
		doc["dns"] = dns
	}
	if *ipv6 {
		dns["strategy"] = "prefer_ipv4"
	} else {
		dns["strategy"] = "ipv4_only"
	}
}

// applyClashIPv6 sets mihomo's top-level and DNS IPv6 switches.
func applyClashIPv6(doc map[string]any, ipv6 *bool) {
	if ipv6 == nil {
		return
	}
	doc["ipv6"] = *ipv6
	if dns, ok := doc["dns"].(map[string]any); ok {
		dns["ipv6"] = *ipv6
	}
}

// sortOptions returns option names that appear in a template, deterministically.
func sortOptions(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
