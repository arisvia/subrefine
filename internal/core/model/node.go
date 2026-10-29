package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Protocol identifiers
const (
	ProtoShadowsocks = "shadowsocks"
	ProtoVLESS       = "vless"
	ProtoVMess       = "vmess"
	ProtoTrojan      = "trojan"
	ProtoHysteria2   = "hysteria2"
	ProtoWireGuard   = "wireguard"
)

// Transport types
const (
	TransportTCP  = "tcp"
	TransportWS   = "ws"
	TransportGRPC = "grpc"
	TransportHTTP = "http"
)

// TLS security types
const (
	SecurityNone    = "none"
	SecurityTLS     = "tls"
	SecurityReality = "reality"
)

// ProxyNode represents a universal parsed proxy node.
type ProxyNode struct {
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Method   string `json:"method,omitempty"` // For Shadowsocks

	// Transport options
	Network     string            `json:"network,omitempty"` // tcp, ws, grpc, etc.
	Path        string            `json:"path,omitempty"`    // WS path, etc.
	Host        string            `json:"host,omitempty"`    // WS host header, etc.
	Headers     map[string]string `json:"headers,omitempty"`
	ServiceName string            `json:"service_name,omitempty"` // gRPC serviceName

	// TLS & Reality options
	Security    string   `json:"security,omitempty"` // tls, reality, none
	ServerName  string   `json:"server_name,omitempty"`
	Insecure    bool     `json:"insecure,omitempty"`
	ALPN        []string `json:"alpn,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"` // uTLS fingerprint (chrome, etc.)
	PublicKey   string   `json:"public_key,omitempty"`  // Reality pbk
	ShortID     string   `json:"short_id,omitempty"`    // Reality sid
	SpiderX     string   `json:"spider_x,omitempty"`    // Reality spx

	// Flow (e.g. xtls-rprx-vision for VLESS)
	Flow string `json:"flow,omitempty"`

	// Hysteria2 options
	UpMbps   int    `json:"up_mbps,omitempty"`
	DownMbps int    `json:"down_mbps,omitempty"`
	ObfsType string `json:"obfs_type,omitempty"`
	ObfsPass string `json:"obfs_pass,omitempty"`

	// WireGuard options
	PrivateKey   string   `json:"private_key,omitempty"`
	LocalAddr    []string `json:"local_addr,omitempty"`
	PeerPubKey   string   `json:"peer_public_key,omitempty"`
	PreSharedKey string   `json:"pre_shared_key,omitempty"`
	MTU          int      `json:"mtu,omitempty"`
	Reserved     []uint8  `json:"reserved,omitempty"`

	// Internal metadata
	Detour string            `json:"detour,omitempty"`
	Extra  map[string]string `json:"extra,omitempty"`
}

// Identity returns a stable short hash of the node's connection parameters.
// It deliberately excludes the tag and any decoration, so pipeline renames
// never move it, and per-key node disable lists survive upstream refreshes.
// If the upstream genuinely changes server/port/credentials, the identity
// changes with the node - stale disable entries then age out.
func (n *ProxyNode) Identity() string {
	sig := strings.Join([]string{
		n.Type, n.Server, strconv.Itoa(n.Port),
		n.UUID, n.Password, n.Method, n.Path,
		n.PrivateKey, n.PeerPubKey, n.PreSharedKey, n.ObfsPass,
	}, "|")
	sum := sha256.Sum256([]byte(sig))
	return hex.EncodeToString(sum[:])[:12]
}

// FilterDisabled drops nodes whose Identity() appears in disabled. It returns
// the original slice untouched when the blacklist is empty, and never mutates
// the input - the full set stays shared in the delivery cache.
func FilterDisabled(nodes []*ProxyNode, disabled []string) []*ProxyNode {
	if len(disabled) == 0 {
		return nodes
	}
	drop := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		drop[d] = true
	}
	out := make([]*ProxyNode, 0, len(nodes))
	for _, n := range nodes {
		if !drop[n.Identity()] {
			out = append(out, n)
		}
	}
	return out
}

// Clone creates a shallow-to-medium clone of the node.
func (n *ProxyNode) Clone() *ProxyNode {
	if n == nil {
		return nil
	}
	cpy := *n
	if len(n.ALPN) > 0 {
		cpy.ALPN = append([]string(nil), n.ALPN...)
	}
	if len(n.LocalAddr) > 0 {
		cpy.LocalAddr = append([]string(nil), n.LocalAddr...)
	}
	if len(n.Reserved) > 0 {
		cpy.Reserved = append([]uint8(nil), n.Reserved...)
	}
	if len(n.Headers) > 0 {
		cpy.Headers = make(map[string]string, len(n.Headers))
		for k, v := range n.Headers {
			cpy.Headers[k] = v
		}
	}
	if len(n.Extra) > 0 {
		cpy.Extra = make(map[string]string, len(n.Extra))
		for k, v := range n.Extra {
			cpy.Extra[k] = v
		}
	}
	return &cpy
}
