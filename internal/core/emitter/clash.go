package emitter

import (
	"fmt"

	"github.com/arisvia/subrefine/internal/core/model"
	"gopkg.in/yaml.v3"
)

// ToClashProxy converts a single ProxyNode into a Clash.Meta compatible map.
func ToClashProxy(n *model.ProxyNode) (map[string]any, error) {
	if n == nil {
		return nil, fmt.Errorf("nil proxy node")
	}

	p := map[string]any{
		"name":   n.Tag,
		"server": n.Server,
		"port":   n.Port,
	}

	switch n.Type {
	case model.ProtoShadowsocks:
		p["type"] = "ss"
		p["cipher"] = n.Method
		p["password"] = n.Password

	case model.ProtoVLESS:
		p["type"] = "vless"
		p["uuid"] = n.UUID
		if n.Flow != "" {
			p["flow"] = n.Flow
		}
		if n.ServerName != "" {
			p["servername"] = n.ServerName
		}
		if n.Fingerprint != "" {
			p["client-fingerprint"] = n.Fingerprint
		}
		if n.Security == model.SecurityTLS || n.Security == model.SecurityReality {
			p["tls"] = true
		}
		if n.Security == model.SecurityReality {
			p["reality-opts"] = map[string]any{
				"public-key": n.PublicKey,
				"short-id":   n.ShortID,
			}
		}
		if n.Network != "" && n.Network != model.TransportTCP {
			p["network"] = n.Network
			if n.Network == model.TransportWS {
				wsOpts := map[string]any{}
				if n.Path != "" {
					wsOpts["path"] = n.Path
				}
				if n.Headers != nil {
					wsOpts["headers"] = n.Headers
				} else if n.Host != "" {
					wsOpts["headers"] = map[string]string{"Host": n.Host}
				}
				p["ws-opts"] = wsOpts
			} else if n.Network == model.TransportGRPC {
				p["grpc-opts"] = map[string]any{
					"grpc-service-name": n.ServiceName,
				}
			}
		}

	case model.ProtoTrojan:
		p["type"] = "trojan"
		p["password"] = n.Password
		if n.ServerName != "" {
			p["sni"] = n.ServerName
		}
		if n.Fingerprint != "" {
			p["client-fingerprint"] = n.Fingerprint
		}
		if n.Insecure {
			p["skip-cert-verify"] = true
		}
		if n.Network != "" && n.Network != model.TransportTCP {
			p["network"] = n.Network
			if n.Network == model.TransportWS {
				wsOpts := map[string]any{}
				if n.Path != "" {
					wsOpts["path"] = n.Path
				}
				p["ws-opts"] = wsOpts
			}
		}

	case model.ProtoHysteria2:
		p["type"] = "hysteria2"
		p["password"] = n.Password
		if n.ServerName != "" {
			p["sni"] = n.ServerName
		}
		if n.Insecure {
			p["skip-cert-verify"] = true
		}
		if n.UpMbps > 0 {
			p["up"] = n.UpMbps
		}
		if n.DownMbps > 0 {
			p["down"] = n.DownMbps
		}
		if n.ObfsType != "" {
			p["obfs"] = n.ObfsType
			p["obfs-password"] = n.ObfsPass
		}
	case model.ProtoVMess:
		p["type"] = "vmess"
		p["uuid"] = n.UUID
		p["alterId"] = 0
		p["cipher"] = "auto"
		if n.ServerName != "" {
			p["servername"] = n.ServerName
		}
		if n.Fingerprint != "" {
			p["client-fingerprint"] = n.Fingerprint
		}
		if n.Security == model.SecurityTLS {
			p["tls"] = true
		}
		if n.Network != "" && n.Network != model.TransportTCP {
			p["network"] = n.Network
			if n.Network == model.TransportWS {
				wsOpts := map[string]any{}
				if n.Path != "" {
					wsOpts["path"] = n.Path
				}
				if n.Headers != nil {
					wsOpts["headers"] = n.Headers
				} else if n.Host != "" {
					wsOpts["headers"] = map[string]string{"Host": n.Host}
				}
				p["ws-opts"] = wsOpts
			}
		}

	case model.ProtoWireGuard:
		p["type"] = "wireguard"
		if n.PeerPubKey != "" {
			p["public-key"] = n.PeerPubKey
		}
		if n.PrivateKey != "" {
			p["private-key"] = n.PrivateKey
		}
		if n.PreSharedKey != "" {
			p["preshared-key"] = n.PreSharedKey
		}
		if len(n.LocalAddr) > 0 {
			p["ip"] = n.LocalAddr[0]
		}
		if n.MTU > 0 {
			p["mtu"] = n.MTU
		}
		p["udp"] = true

	default:
		return nil, fmt.Errorf("unsupported clash proxy type: %s", n.Type)
	}

	return p, nil
}

// ToClashYAML serializes nodes into a complete Clash.Meta configuration YAML.
func ToClashYAML(nodes []*model.ProxyNode) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(nodes))
	names := make([]string, 0, len(nodes))

	for _, n := range nodes {
		p, err := ToClashProxy(n)
		if err == nil && p != nil {
			proxies = append(proxies, p)
			if name, ok := p["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}

	// A subscription filtered to zero nodes must still be a loadable config:
	// mihomo rejects a url-test group with no proxies ("use or proxies
	// missing"), so fall back to a bare DIRECT rule set instead of emitting
	// empty AUTO/PROXIES groups.
	if len(names) == 0 {
		doc := map[string]any{
			"proxies":      []map[string]any{},
			"proxy-groups": []map[string]any{},
			"rules":        []string{"MATCH,DIRECT"},
		}
		return yaml.Marshal(doc)
	}

	// Basic group structure
	groups := []map[string]any{
		{
			"name":    "PROXIES",
			"type":    "select",
			"proxies": append([]string{"AUTO"}, names...),
		},
		{
			"name":     "AUTO",
			"type":     "url-test",
			"url":      "https://cp.cloudflare.com/generate_204",
			"interval": 300,
			"proxies":  names,
		},
	}

	doc := map[string]any{
		"proxies":      proxies,
		"proxy-groups": groups,
		"rules": []string{
			"MATCH,PROXIES",
		},
	}

	return yaml.Marshal(doc)
}
