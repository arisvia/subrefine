package emitter

import (
	"encoding/json"
	"fmt"

	"github.com/arisvia/subrefine/internal/core/model"
)

// ToSingboxOutbound converts a single ProxyNode into a sing-box 1.15 outbound map.
func ToSingboxOutbound(n *model.ProxyNode) (map[string]any, error) {
	if n == nil {
		return nil, fmt.Errorf("nil proxy node")
	}

	out := map[string]any{
		"type":        n.Type,
		"tag":         n.Tag,
		"server":      n.Server,
		"server_port": n.Port,
	}

	switch n.Type {
	case model.ProtoShadowsocks:
		out["method"] = n.Method
		out["password"] = n.Password

	case model.ProtoVLESS:
		out["uuid"] = n.UUID
		if n.Flow != "" {
			out["flow"] = n.Flow
		}

		if n.Security == model.SecurityTLS || n.Security == model.SecurityReality {
			tls := map[string]any{
				"enabled": true,
			}
			if n.ServerName != "" {
				tls["server_name"] = n.ServerName
			}
			if n.Insecure {
				tls["insecure"] = true
			}
			if len(n.ALPN) > 0 {
				tls["alpn"] = n.ALPN
			}
			if n.Fingerprint != "" {
				tls["utls"] = map[string]any{
					"enabled":     true,
					"fingerprint": n.Fingerprint,
				}
			}
			if n.Security == model.SecurityReality {
				tls["reality"] = map[string]any{
					"enabled":    true,
					"public_key": n.PublicKey,
					"short_id":   n.ShortID,
				}
			}
			out["tls"] = tls
		}

		if n.Network != "" && n.Network != model.TransportTCP {
			tr := map[string]any{
				"type": n.Network,
			}
			if n.Path != "" {
				tr["path"] = n.Path
			}
			if n.Headers != nil {
				tr["headers"] = n.Headers
			} else if n.Host != "" {
				tr["headers"] = map[string]string{"Host": n.Host}
			}
			if n.ServiceName != "" {
				tr["service_name"] = n.ServiceName
			}
			out["transport"] = tr
		}

	case model.ProtoTrojan:
		out["password"] = n.Password
		tls := map[string]any{
			"enabled": true,
		}
		if n.ServerName != "" {
			tls["server_name"] = n.ServerName
		}
		if n.Insecure {
			tls["insecure"] = true
		}
		if len(n.ALPN) > 0 {
			tls["alpn"] = n.ALPN
		}
		if n.Fingerprint != "" {
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": n.Fingerprint,
			}
		}
		out["tls"] = tls

		if n.Network != "" && n.Network != model.TransportTCP {
			tr := map[string]any{
				"type": n.Network,
			}
			if n.Path != "" {
				tr["path"] = n.Path
			}
			if n.Headers != nil {
				tr["headers"] = n.Headers
			} else if n.Host != "" {
				tr["headers"] = map[string]string{"Host": n.Host}
			}
			out["transport"] = tr
		}

	case model.ProtoHysteria2:
		out["password"] = n.Password
		if n.UpMbps > 0 {
			out["up_mbps"] = n.UpMbps
		}
		if n.DownMbps > 0 {
			out["down_mbps"] = n.DownMbps
		}

		tls := map[string]any{
			"enabled": true,
		}
		if n.ServerName != "" {
			tls["server_name"] = n.ServerName
		}
		if n.Insecure {
			tls["insecure"] = true
		}
		if len(n.ALPN) > 0 {
			tls["alpn"] = n.ALPN
		}
		out["tls"] = tls

		if n.ObfsType != "" {
			out["obfs"] = map[string]any{
				"type":     n.ObfsType,
				"password": n.ObfsPass,
			}
		}
	case model.ProtoVMess:
		out["uuid"] = n.UUID
		out["security"] = "auto"
		if n.Security == model.SecurityTLS {
			tls := map[string]any{
				"enabled": true,
			}
			if n.ServerName != "" {
				tls["server_name"] = n.ServerName
			}
			if n.Insecure {
				tls["insecure"] = true
			}
			if len(n.ALPN) > 0 {
				tls["alpn"] = n.ALPN
			}
			if n.Fingerprint != "" {
				tls["utls"] = map[string]any{
					"enabled":     true,
					"fingerprint": n.Fingerprint,
				}
			}
			out["tls"] = tls
		}
		if n.Network != "" && n.Network != model.TransportTCP {
			tr := map[string]any{
				"type": n.Network,
			}
			if n.Path != "" {
				tr["path"] = n.Path
			}
			if n.Headers != nil {
				tr["headers"] = n.Headers
			} else if n.Host != "" {
				tr["headers"] = map[string]string{"Host": n.Host}
			}
			out["transport"] = tr
		}

	case model.ProtoWireGuard:
		// sing-box 1.13.0 removed the wireguard outbound and its endpoint
		// replacement is a startup tunnel, not a per-node proxy, so WG cannot
		// be a selectable outbound in any current sing-box. The delivery
		// handler announces the skip; clash and uri targets still carry WG.
		return nil, fmt.Errorf("wireguard is not a sing-box outbound since 1.13.0")

	default:
		return nil, fmt.Errorf("unsupported sing-box outbound type: %s", n.Type)
	}

	return out, nil
}

// SingboxSkipped lists nodes the sing-box target deliberately drops:
// WireGuard has been an invalid outbound since sing-box 1.13.0, and an
// undeliverable node must be announced, never silently vanished.
func SingboxSkipped(nodes []*model.ProxyNode) []string {
	var tags []string
	for _, n := range nodes {
		if n != nil && n.Type == model.ProtoWireGuard {
			tags = append(tags, n.Tag)
		}
	}
	return tags
}

// ToSingboxJSON serializes a list of ProxyNodes into a sing-box config JSON.
// Nodes the target cannot express (WireGuard since sing-box 1.13.0) are left
// out; callers announce that through SingboxSkipped.
func ToSingboxJSON(nodes []*model.ProxyNode) ([]byte, error) {
	outbounds := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		ob, err := ToSingboxOutbound(n)
		if err == nil && ob != nil {
			outbounds = append(outbounds, ob)
		}
	}

	doc := map[string]any{
		"outbounds": outbounds,
	}

	return json.MarshalIndent(doc, "", "  ")
}
