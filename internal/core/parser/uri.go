package parser

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported protocol scheme")
	ErrInvalidURI        = errors.New("invalid proxy uri")
)

// ParseURI parses a single proxy URI into a universal ProxyNode.
func ParseURI(raw string) (*model.ProxyNode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURI
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURI, err)
	}

	tag := u.Fragment
	if unescaped, err := url.QueryUnescape(tag); err == nil && unescaped != "" {
		tag = unescaped
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "ss":
		return parseShadowsocks(u, tag)
	case "vless":
		return parseVLESS(u, tag)
	case "trojan":
		return parseTrojan(u, tag)
	case "hy2", "hysteria2":
		return parseHysteria2(u, tag)
	case "vmess":
		return parseVMess(raw, tag)
	case "wg", "wireguard":
		return parseWireGuard(u, tag)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedScheme, u.Scheme)
	}
}

func parsePort(portStr string) (int, error) {
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 || p > 65535 {
		return 0, fmt.Errorf("invalid port: %s", portStr)
	}
	return p, nil
}

func parseShadowsocks(u *url.URL, tag string) (*model.ProxyNode, error) {
	node := &model.ProxyNode{
		Type: model.ProtoShadowsocks,
		Tag:  tag,
	}

	if u.User == nil {
		rawStr := u.Host
		decoded, err := decodeBase64Safe(rawStr)
		if err == nil && strings.Contains(string(decoded), "@") {
			reparsed, err := url.Parse("ss://" + string(decoded))
			if err == nil {
				return parseShadowsocks(reparsed, tag)
			}
		}
		return nil, fmt.Errorf("%w: invalid ss format", ErrInvalidURI)
	}

	userinfo := u.User.String()
	if !strings.Contains(userinfo, ":") {
		decoded, err := decodeBase64Safe(userinfo)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid ss userinfo base64", ErrInvalidURI)
		}
		userinfo = string(decoded)
	}

	parts := strings.SplitN(userinfo, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: invalid ss method/password", ErrInvalidURI)
	}
	node.Method = parts[0]
	node.Password = parts[1]

	node.Server = u.Hostname()
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}
	node.Port = port

	if node.Tag == "" {
		node.Tag = fmt.Sprintf("SS-%s:%d", node.Server, node.Port)
	}
	return node, nil
}

func parseVLESS(u *url.URL, tag string) (*model.ProxyNode, error) {
	node := &model.ProxyNode{
		Type:   model.ProtoVLESS,
		Tag:    tag,
		UUID:   u.User.Username(),
		Server: u.Hostname(),
	}

	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}
	node.Port = port

	q := u.Query()
	node.Network = q.Get("type")
	if node.Network == "" {
		node.Network = model.TransportTCP
	}

	node.Security = q.Get("security")
	if node.Security == "" {
		node.Security = model.SecurityNone
	}

	node.Flow = q.Get("flow")
	node.ServerName = q.Get("sni")
	node.Fingerprint = q.Get("fp")
	node.PublicKey = q.Get("pbk")
	node.ShortID = q.Get("sid")
	node.SpiderX = q.Get("spx")
	node.Path = q.Get("path")
	node.Host = q.Get("host")
	node.ServiceName = q.Get("serviceName")

	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}

	if node.Tag == "" {
		node.Tag = fmt.Sprintf("VLESS-%s:%d", node.Server, node.Port)
	}
	return node, nil
}

func parseTrojan(u *url.URL, tag string) (*model.ProxyNode, error) {
	node := &model.ProxyNode{
		Type:     model.ProtoTrojan,
		Tag:      tag,
		Password: u.User.Username(),
		Server:   u.Hostname(),
	}

	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}
	node.Port = port

	q := u.Query()
	node.Network = q.Get("type")
	if node.Network == "" {
		node.Network = model.TransportTCP
	}

	node.Security = model.SecurityTLS
	if sec := q.Get("security"); sec != "" {
		node.Security = sec
	}

	node.ServerName = q.Get("sni")
	if node.ServerName == "" {
		node.ServerName = q.Get("peer")
	}
	node.Fingerprint = q.Get("fp")
	node.Path = q.Get("path")
	node.Host = q.Get("host")
	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}
	if q.Get("allowInsecure") == "1" || q.Get("insecure") == "1" {
		node.Insecure = true
	}

	if node.Tag == "" {
		node.Tag = fmt.Sprintf("Trojan-%s:%d", node.Server, node.Port)
	}
	return node, nil
}

func parseHysteria2(u *url.URL, tag string) (*model.ProxyNode, error) {
	node := &model.ProxyNode{
		Type:     model.ProtoHysteria2,
		Tag:      tag,
		Password: u.User.Username(),
		Server:   u.Hostname(),
		Security: model.SecurityTLS,
	}

	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}
	node.Port = port

	q := u.Query()
	node.ServerName = q.Get("sni")
	if q.Get("insecure") == "1" {
		node.Insecure = true
	}

	if obfs := q.Get("obfs"); obfs != "" {
		node.ObfsType = obfs
		node.ObfsPass = q.Get("obfs-password")
	}

	if node.Tag == "" {
		node.Tag = fmt.Sprintf("Hy2-%s:%d", node.Server, node.Port)
	}
	return node, nil
}

func parseVMess(raw, tag string) (*model.ProxyNode, error) {
	b64 := strings.TrimPrefix(raw, "vmess://")
	if idx := strings.Index(b64, "#"); idx != -1 {
		b64 = b64[:idx]
	}
	decoded, err := decodeBase64Safe(b64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid vmess base64: %v", ErrInvalidURI, err)
	}

	var v struct {
		V    any    `json:"v"`
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		Alpn string `json:"alpn"`
		Fp   string `json:"fp"`
	}
	if err := json.Unmarshal(decoded, &v); err != nil {
		return nil, fmt.Errorf("%w: invalid vmess json: %v", ErrInvalidURI, err)
	}

	port, err := parsePort(fmt.Sprint(v.Port))
	if err != nil {
		return nil, err
	}

	nodeTag := tag
	if nodeTag == "" {
		nodeTag = v.PS
	}
	if nodeTag == "" {
		nodeTag = fmt.Sprintf("VMess-%s:%d", v.Add, port)
	}

	node := &model.ProxyNode{
		Tag:         nodeTag,
		Type:        model.ProtoVMess,
		Server:      v.Add,
		Port:        port,
		UUID:        v.ID,
		Network:     v.Net,
		Path:        v.Path,
		Host:        v.Host,
		Fingerprint: v.Fp,
	}
	if node.Network == "" {
		node.Network = model.TransportTCP
	}
	if v.TLS == "tls" {
		node.Security = model.SecurityTLS
		node.ServerName = v.SNI
		if node.ServerName == "" {
			node.ServerName = v.Host
		}
		if v.Alpn != "" {
			node.ALPN = strings.Split(v.Alpn, ",")
		}
	}

	return node, nil
}

func parseWireGuard(u *url.URL, tag string) (*model.ProxyNode, error) {
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, err
	}

	q := u.Query()
	node := &model.ProxyNode{
		Tag:    tag,
		Type:   model.ProtoWireGuard,
		Server: u.Hostname(),
		Port:   port,
	}
	if u.User != nil {
		node.PeerPubKey = u.User.Username()
	}
	if node.PeerPubKey == "" {
		node.PeerPubKey = q.Get("publickey")
	}
	node.PrivateKey = q.Get("privatekey")
	if node.PrivateKey == "" {
		node.PrivateKey = q.Get("secretkey")
	}
	if ip := q.Get("ip"); ip != "" {
		node.LocalAddr = strings.Split(ip, ",")
	}
	if mtuStr := q.Get("mtu"); mtuStr != "" {
		if mtu, err := strconv.Atoi(mtuStr); err == nil {
			node.MTU = mtu
		}
	}
	node.PreSharedKey = q.Get("presharedkey")

	if node.Tag == "" {
		node.Tag = fmt.Sprintf("WG-%s:%d", node.Server, node.Port)
	}
	return node, nil
}

func decodeBase64Safe(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}
