package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

type singboxRoot struct {
	Outbounds []json.RawMessage `json:"outbounds"`
}

type singboxHeader struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

// ParseSingbox parses a sing-box config JSON or outbounds array into ProxyNodes.
func ParseSingbox(data []byte) ([]*model.ProxyNode, error) {
	// Try parsing full sing-box config with "outbounds" key
	var root singboxRoot
	var rawOutbounds []json.RawMessage

	if err := json.Unmarshal(data, &root); err == nil && len(root.Outbounds) > 0 {
		rawOutbounds = root.Outbounds
	} else {
		// Try parsing as raw array of outbounds
		if err := json.Unmarshal(data, &rawOutbounds); err != nil {
			return nil, fmt.Errorf("invalid sing-box json: %w", err)
		}
	}

	var nodes []*model.ProxyNode
	for _, raw := range rawOutbounds {
		var header singboxHeader
		if err := json.Unmarshal(raw, &header); err != nil {
			continue
		}

		// Filter out internal non-proxy outbounds
		switch header.Type {
		case "direct", "block", "dns", "selector", "urltest":
			continue
		}

		node, err := parseSingboxOutbound(header.Type, raw)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func parseSingboxOutbound(proto string, raw []byte) (*model.ProxyNode, error) {
	switch proto {
	case "shadowsocks":
		var s struct {
			Tag      string `json:"tag"`
			Server   string `json:"server"`
			Port     int    `json:"server_port"`
			Method   string `json:"method"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &model.ProxyNode{
			Tag:      s.Tag,
			Type:     model.ProtoShadowsocks,
			Server:   s.Server,
			Port:     s.Port,
			Method:   s.Method,
			Password: s.Password,
		}, nil

	case "vless":
		var v struct {
			Tag     string `json:"tag"`
			Server  string `json:"server"`
			Port    int    `json:"server_port"`
			UUID    string `json:"uuid"`
			Flow    string `json:"flow"`
			Network string `json:"network"`
			TLS     *struct {
				Enabled    bool     `json:"enabled"`
				ServerName string   `json:"server_name"`
				Insecure   bool     `json:"insecure"`
				ALPN       []string `json:"alpn"`
				UTLS       *struct {
					Enabled     bool   `json:"enabled"`
					Fingerprint string `json:"fingerprint"`
				} `json:"utls"`
				Reality *struct {
					Enabled   bool   `json:"enabled"`
					PublicKey string `json:"public_key"`
					ShortID   string `json:"short_id"`
				} `json:"reality"`
			} `json:"tls"`
			Transport *struct {
				Type        string            `json:"type"`
				Path        string            `json:"path"`
				Headers     map[string]string `json:"headers"`
				ServiceName string            `json:"service_name"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}

		node := &model.ProxyNode{
			Tag:     v.Tag,
			Type:    model.ProtoVLESS,
			Server:  v.Server,
			Port:    v.Port,
			UUID:    v.UUID,
			Flow:    v.Flow,
			Network: model.TransportTCP,
		}

		if v.TLS != nil && v.TLS.Enabled {
			node.Security = model.SecurityTLS
			node.ServerName = v.TLS.ServerName
			node.Insecure = v.TLS.Insecure
			node.ALPN = v.TLS.ALPN
			if v.TLS.UTLS != nil && v.TLS.UTLS.Enabled {
				node.Fingerprint = v.TLS.UTLS.Fingerprint
			}
			if v.TLS.Reality != nil && v.TLS.Reality.Enabled {
				node.Security = model.SecurityReality
				node.PublicKey = v.TLS.Reality.PublicKey
				node.ShortID = v.TLS.Reality.ShortID
			}
		}

		if v.Transport != nil {
			node.Network = v.Transport.Type
			node.Path = v.Transport.Path
			node.Headers = v.Transport.Headers
			node.ServiceName = v.Transport.ServiceName
			if v.Transport.Headers != nil && v.Transport.Headers["Host"] != "" {
				node.Host = v.Transport.Headers["Host"]
			}
		}

		return node, nil

	case "trojan":
		var t struct {
			Tag      string `json:"tag"`
			Server   string `json:"server"`
			Port     int    `json:"server_port"`
			Password string `json:"password"`
			TLS      *struct {
				Enabled    bool     `json:"enabled"`
				ServerName string   `json:"server_name"`
				Insecure   bool     `json:"insecure"`
				ALPN       []string `json:"alpn"`
				UTLS       *struct {
					Fingerprint string `json:"fingerprint"`
				} `json:"utls"`
			} `json:"tls"`
			Transport *struct {
				Type    string            `json:"type"`
				Path    string            `json:"path"`
				Headers map[string]string `json:"headers"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, err
		}

		node := &model.ProxyNode{
			Tag:      t.Tag,
			Type:     model.ProtoTrojan,
			Server:   t.Server,
			Port:     t.Port,
			Password: t.Password,
			Security: model.SecurityTLS,
			Network:  model.TransportTCP,
		}
		if t.TLS != nil {
			node.ServerName = t.TLS.ServerName
			node.Insecure = t.TLS.Insecure
			node.ALPN = t.TLS.ALPN
			if t.TLS.UTLS != nil {
				node.Fingerprint = t.TLS.UTLS.Fingerprint
			}
		}
		if t.Transport != nil {
			node.Network = t.Transport.Type
			node.Path = t.Transport.Path
			node.Headers = t.Transport.Headers
			if t.Transport.Headers != nil && t.Transport.Headers["Host"] != "" {
				node.Host = t.Transport.Headers["Host"]
			}
		}
		return node, nil

	case "hysteria2":
		var h struct {
			Tag      string `json:"tag"`
			Server   string `json:"server"`
			Port     int    `json:"server_port"`
			Password string `json:"password"`
			UpMbps   int    `json:"up_mbps"`
			DownMbps int    `json:"down_mbps"`
			TLS      *struct {
				Enabled    bool     `json:"enabled"`
				ServerName string   `json:"server_name"`
				Insecure   bool     `json:"insecure"`
				ALPN       []string `json:"alpn"`
			} `json:"tls"`
			Obfs *struct {
				Type     string `json:"type"`
				Password string `json:"password"`
			} `json:"obfs"`
		}
		if err := json.Unmarshal(raw, &h); err != nil {
			return nil, err
		}

		node := &model.ProxyNode{
			Tag:      h.Tag,
			Type:     model.ProtoHysteria2,
			Server:   h.Server,
			Port:     h.Port,
			Password: h.Password,
			UpMbps:   h.UpMbps,
			DownMbps: h.DownMbps,
			Security: model.SecurityTLS,
		}
		if h.TLS != nil {
			node.ServerName = h.TLS.ServerName
			node.Insecure = h.TLS.Insecure
			node.ALPN = h.TLS.ALPN
		}
		if h.Obfs != nil {
			node.ObfsType = h.Obfs.Type
			node.ObfsPass = h.Obfs.Password
		}
		return node, nil

	case "vmess":
		var vm struct {
			Tag      string `json:"tag"`
			Server   string `json:"server"`
			Port     int    `json:"server_port"`
			UUID     string `json:"uuid"`
			AlterId  int    `json:"alter_id"`
			Security string `json:"security"`
			TLS      *struct {
				Enabled    bool     `json:"enabled"`
				ServerName string   `json:"server_name"`
				Insecure   bool     `json:"insecure"`
				ALPN       []string `json:"alpn"`
				UTLS       *struct {
					Fingerprint string `json:"fingerprint"`
				} `json:"utls"`
			} `json:"tls"`
			Transport *struct {
				Type    string            `json:"type"`
				Path    string            `json:"path"`
				Headers map[string]string `json:"headers"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(raw, &vm); err != nil {
			return nil, err
		}
		node := &model.ProxyNode{
			Tag:     vm.Tag,
			Type:    model.ProtoVMess,
			Server:  vm.Server,
			Port:    vm.Port,
			UUID:    vm.UUID,
			Network: model.TransportTCP,
		}
		if vm.TLS != nil && vm.TLS.Enabled {
			node.Security = model.SecurityTLS
			node.ServerName = vm.TLS.ServerName
			node.Insecure = vm.TLS.Insecure
			node.ALPN = vm.TLS.ALPN
			if vm.TLS.UTLS != nil {
				node.Fingerprint = vm.TLS.UTLS.Fingerprint
			}
		}
		if vm.Transport != nil {
			node.Network = vm.Transport.Type
			node.Path = vm.Transport.Path
			node.Headers = vm.Transport.Headers
			if vm.Transport.Headers != nil && vm.Transport.Headers["Host"] != "" {
				node.Host = vm.Transport.Headers["Host"]
			}
		}
		return node, nil

	case "wireguard":
		var wg struct {
			Tag          string   `json:"tag"`
			Server       string   `json:"server"`
			Port         int      `json:"server_port"`
			PrivateKey   string   `json:"private_key"`
			PeerPubKey   string   `json:"peer_public_key"`
			PreSharedKey string   `json:"pre_shared_key"`
			LocalAddr    []string `json:"local_address"`
			MTU          int      `json:"mtu"`
			Reserved     []uint8  `json:"reserved"`
		}
		if err := json.Unmarshal(raw, &wg); err != nil {
			return nil, err
		}
		return &model.ProxyNode{
			Tag:          wg.Tag,
			Type:         model.ProtoWireGuard,
			Server:       wg.Server,
			Port:         wg.Port,
			PrivateKey:   wg.PrivateKey,
			PeerPubKey:   wg.PeerPubKey,
			PreSharedKey: wg.PreSharedKey,
			LocalAddr:    wg.LocalAddr,
			MTU:          wg.MTU,
			Reserved:     wg.Reserved,
		}, nil
	}
	return nil, fmt.Errorf("unsupported sing-box outbound protocol: %s", strings.ToLower(proto))
}
