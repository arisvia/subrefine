package emitter

import (
	"encoding/json"
	"fmt"
	"subrefine/internal/core/model"
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
		if n.PrivateKey != "" {
			out["private_key"] = n.PrivateKey
		}
		if n.PeerPubKey != "" {
			out["peer_public_key"] = n.PeerPubKey
		}
		if n.PreSharedKey != "" {
			out["pre_shared_key"] = n.PreSharedKey
		}
		if len(n.LocalAddr) > 0 {
			out["local_address"] = n.LocalAddr
		}
		if n.MTU > 0 {
			out["mtu"] = n.MTU
		}
		if len(n.Reserved) > 0 {
			out["reserved"] = n.Reserved
		}

	default:
		return nil, fmt.Errorf("unsupported sing-box outbound type: %s", n.Type)
	}

	return out, nil
}

// ToSingboxJSON serializes a list of ProxyNodes into a sing-box config JSON.
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
