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

	case model.ProtoAnyTLS:
		p["type"] = "anytls"
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
		if len(n.ALPN) > 0 {
			p["alpn"] = n.ALPN
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

// ToClashYAML serializes nodes into a bare Clash.Meta proxy list — the
// fragment form. It carries ONLY the nodes (no proxy-groups, no rules, no
// listeners): a fragment is meant to be imported as a provider or merged into
// a client's own config, so anything beyond the node list is noise. The
// complete, loadable configuration (groups, rules, inbounds) is the
// full-template render path, not this one.
func ToClashYAML(nodes []*model.ProxyNode) ([]byte, error) {
	proxies := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		if p, err := ToClashProxy(n); err == nil && p != nil {
			proxies = append(proxies, p)
		}
	}
	return yaml.Marshal(map[string]any{"proxies": proxies})
}
