package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"subrefine/internal/core/model"
)

type clashFile struct {
	Proxies []map[string]any `yaml:"proxies"`
}

// ParseClash parses Clash / Clash.Meta YAML into ProxyNodes.
func ParseClash(data []byte) ([]*model.ProxyNode, error) {
	var cf clashFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		// Try parsing as array directly
		var arr []map[string]any
		if err2 := yaml.Unmarshal(data, &arr); err2 != nil {
			return nil, fmt.Errorf("invalid clash yaml: %w", err)
		}
		cf.Proxies = arr
	}

	var nodes []*model.ProxyNode
	for _, p := range cf.Proxies {
		node, err := parseClashProxy(p)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func parseClashProxy(m map[string]any) (*model.ProxyNode, error) {
	proto := strings.ToLower(getString(m, "type"))
	tag := getString(m, "name")
	server := getString(m, "server")
	port := getInt(m, "port")

	if server == "" || port <= 0 {
		return nil, fmt.Errorf("missing server or port")
	}

	switch proto {
	case "ss":
		return &model.ProxyNode{
			Tag:      tag,
			Type:     model.ProtoShadowsocks,
			Server:   server,
			Port:     port,
			Method:   getString(m, "cipher"),
			Password: getString(m, "password"),
		}, nil

	case "vless":
		node := &model.ProxyNode{
			Tag:         tag,
			Type:        model.ProtoVLESS,
			Server:      server,
			Port:        port,
			UUID:        getString(m, "uuid"),
			Flow:        getString(m, "flow"),
			Network:     model.TransportTCP,
			Fingerprint: getString(m, "client-fingerprint"),
			ServerName:  getString(m, "servername"),
		}
		if node.ServerName == "" {
			node.ServerName = getString(m, "sni")
		}

		if getBool(m, "tls") {
			node.Security = model.SecurityTLS
		}
		if realityOpts, ok := m["reality-opts"].(map[string]any); ok {
			node.Security = model.SecurityReality
			node.PublicKey = getString(realityOpts, "public-key")
			node.ShortID = getString(realityOpts, "short-id")
		}
		if net := getString(m, "network"); net != "" {
			node.Network = net
		}
		if wsOpts, ok := m["ws-opts"].(map[string]any); ok {
			node.Path = getString(wsOpts, "path")
			if headers, ok := wsOpts["headers"].(map[string]any); ok {
				node.Headers = make(map[string]string)
				for k, v := range headers {
					node.Headers[k] = fmt.Sprint(v)
				}
				if h, ok := node.Headers["Host"]; ok {
					node.Host = h
				}
			}
		}
		if grpcOpts, ok := m["grpc-opts"].(map[string]any); ok {
			node.ServiceName = getString(grpcOpts, "grpc-service-name")
		}
		return node, nil

	case "trojan":
		node := &model.ProxyNode{
			Tag:         tag,
			Type:        model.ProtoTrojan,
			Server:      server,
			Port:        port,
			Password:    getString(m, "password"),
			Security:    model.SecurityTLS,
			Fingerprint: getString(m, "client-fingerprint"),
			ServerName:  getString(m, "sni"),
			Insecure:    getBool(m, "skip-cert-verify"),
			Network:     model.TransportTCP,
		}
		if net := getString(m, "network"); net != "" {
			node.Network = net
		}
		if wsOpts, ok := m["ws-opts"].(map[string]any); ok {
			node.Path = getString(wsOpts, "path")
		}
		return node, nil

	case "hysteria2":
		node := &model.ProxyNode{
			Tag:        tag,
			Type:       model.ProtoHysteria2,
			Server:     server,
			Port:       port,
			Password:   getString(m, "password"),
			Security:   model.SecurityTLS,
			ServerName: getString(m, "sni"),
			Insecure:   getBool(m, "skip-cert-verify"),
			UpMbps:     getInt(m, "up"),
			DownMbps:   getInt(m, "down"),
			ObfsType:   getString(m, "obfs"),
			ObfsPass:   getString(m, "obfs-password"),
		}
		return node, nil

	case "vmess":
		node := &model.ProxyNode{
			Tag:         tag,
			Type:        model.ProtoVMess,
			Server:      server,
			Port:        port,
			UUID:        getString(m, "uuid"),
			Fingerprint: getString(m, "client-fingerprint"),
			ServerName:  getString(m, "servername"),
			Network:     model.TransportTCP,
		}
		if node.ServerName == "" {
			node.ServerName = getString(m, "sni")
		}
		if getBool(m, "tls") {
			node.Security = model.SecurityTLS
		}
		if net := getString(m, "network"); net != "" {
			node.Network = net
		}
		if wsOpts, ok := m["ws-opts"].(map[string]any); ok {
			node.Path = getString(wsOpts, "path")
			if headers, ok := wsOpts["headers"].(map[string]any); ok {
				node.Headers = make(map[string]string)
				for k, v := range headers {
					node.Headers[k] = fmt.Sprint(v)
				}
				if h, ok := node.Headers["Host"]; ok {
					node.Host = h
				}
			}
		}
		return node, nil

	case "wireguard":
		node := &model.ProxyNode{
			Tag:          tag,
			Type:         model.ProtoWireGuard,
			Server:       server,
			Port:         port,
			PeerPubKey:   getString(m, "public-key"),
			PrivateKey:   getString(m, "private-key"),
			PreSharedKey: getString(m, "preshared-key"),
			MTU:          getInt(m, "mtu"),
		}
		if ip := getString(m, "ip"); ip != "" {
			node.LocalAddr = []string{ip}
		}
		return node, nil
	}
	return nil, fmt.Errorf("unsupported clash proxy type: %s", proto)
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var n int
			fmt.Sscanf(val, "%d", &n)
			return n
		}
	}
	return 0
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
		if s, ok := v.(string); ok {
			return strings.ToLower(s) == "true" || s == "1"
		}
	}
	return false
}
