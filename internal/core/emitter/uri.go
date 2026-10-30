package emitter

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

var (
	ErrUnsupportedNode = errors.New("unsupported node type for URI emitter")
)

// ToURI converts a ProxyNode to its standard URI representation.
func ToURI(n *model.ProxyNode) (string, error) {
	if n == nil {
		return "", errors.New("nil proxy node")
	}

	switch n.Type {
	case model.ProtoShadowsocks:
		return emitShadowsocks(n)
	case model.ProtoVLESS:
		return emitVLESS(n)
	case model.ProtoTrojan:
		return emitTrojan(n)
	case model.ProtoAnyTLS:
		return emitAnyTLS(n)
	case model.ProtoHysteria2:
		return emitHysteria2(n)
	case model.ProtoVMess:
		return emitVMess(n)
	case model.ProtoWireGuard:
		return emitWireGuard(n)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedNode, n.Type)
	}
}

func emitShadowsocks(n *model.ProxyNode) (string, error) {
	userinfo := n.Method + ":" + n.Password
	encodedUser := base64.URLEncoding.EncodeToString([]byte(userinfo))
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))

	res := fmt.Sprintf("ss://%s@%s", encodedUser, hostPort)
	if n.Tag != "" {
		res += "#" + url.QueryEscape(n.Tag)
	}
	return res, nil
}

func emitVLESS(n *model.ProxyNode) (string, error) {
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	v := url.Values{}

	if n.Network != "" && n.Network != model.TransportTCP {
		v.Set("type", n.Network)
	}
	if n.Security != "" && n.Security != model.SecurityNone {
		v.Set("security", n.Security)
	}
	if n.Flow != "" {
		v.Set("flow", n.Flow)
	}
	if n.ServerName != "" {
		v.Set("sni", n.ServerName)
	}
	if n.Fingerprint != "" {
		v.Set("fp", n.Fingerprint)
	}
	if n.PublicKey != "" {
		v.Set("pbk", n.PublicKey)
	}
	if n.ShortID != "" {
		v.Set("sid", n.ShortID)
	}
	if n.SpiderX != "" {
		v.Set("spx", n.SpiderX)
	}
	if n.Path != "" {
		v.Set("path", n.Path)
	}
	if n.Host != "" {
		v.Set("host", n.Host)
	}
	if n.ServiceName != "" {
		v.Set("serviceName", n.ServiceName)
	}
	if len(n.ALPN) > 0 {
		v.Set("alpn", strings.Join(n.ALPN, ","))
	}

	query := v.Encode()
	uri := fmt.Sprintf("vless://%s@%s", n.UUID, hostPort)
	if query != "" {
		uri += "?" + query
	}
	if n.Tag != "" {
		uri += "#" + url.QueryEscape(n.Tag)
	}
	return uri, nil
}

func emitTrojan(n *model.ProxyNode) (string, error) {
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	v := url.Values{}

	if n.Network != "" && n.Network != model.TransportTCP {
		v.Set("type", n.Network)
	}
	if n.Security != "" && n.Security != model.SecurityTLS {
		v.Set("security", n.Security)
	}
	if n.ServerName != "" {
		v.Set("sni", n.ServerName)
	}
	if n.Fingerprint != "" {
		v.Set("fp", n.Fingerprint)
	}
	if n.Insecure {
		v.Set("allowInsecure", "1")
	}
	if n.Path != "" {
		v.Set("path", n.Path)
	}
	if n.Host != "" {
		v.Set("host", n.Host)
	}
	if len(n.ALPN) > 0 {
		v.Set("alpn", strings.Join(n.ALPN, ","))
	}

	query := v.Encode()
	uri := fmt.Sprintf("trojan://%s@%s", url.QueryEscape(n.Password), hostPort)
	if query != "" {
		uri += "?" + query
	}
	if n.Tag != "" {
		uri += "#" + url.QueryEscape(n.Tag)
	}
	return uri, nil
}

// emitAnyTLS writes the anytls:// URI form. AnyTLS is TLS-fixed, so only the
// handshake parameters round-trip.
func emitAnyTLS(n *model.ProxyNode) (string, error) {
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	v := url.Values{}

	if n.ServerName != "" {
		v.Set("sni", n.ServerName)
	}
	if n.Fingerprint != "" {
		v.Set("fp", n.Fingerprint)
	}
	if n.Insecure {
		v.Set("insecure", "1")
	}
	if len(n.ALPN) > 0 {
		v.Set("alpn", strings.Join(n.ALPN, ","))
	}

	query := v.Encode()
	uri := fmt.Sprintf("anytls://%s@%s", url.QueryEscape(n.Password), hostPort)
	if query != "" {
		uri += "?" + query
	}
	if n.Tag != "" {
		uri += "#" + url.QueryEscape(n.Tag)
	}
	return uri, nil
}

func emitHysteria2(n *model.ProxyNode) (string, error) {
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	v := url.Values{}

	if n.ServerName != "" {
		v.Set("sni", n.ServerName)
	}
	if n.Insecure {
		v.Set("insecure", "1")
	}
	if n.ObfsType != "" {
		v.Set("obfs", n.ObfsType)
		if n.ObfsPass != "" {
			v.Set("obfs-password", n.ObfsPass)
		}
	}

	query := v.Encode()
	uri := fmt.Sprintf("hysteria2://%s@%s", url.QueryEscape(n.Password), hostPort)
	if query != "" {
		uri += "?" + query
	}
	if n.Tag != "" {
		uri += "#" + url.QueryEscape(n.Tag)
	}
	return uri, nil
}

func emitVMess(n *model.ProxyNode) (string, error) {
	v := map[string]any{
		"v":    "2",
		"ps":   n.Tag,
		"add":  n.Server,
		"port": n.Port,
		"id":   n.UUID,
		"aid":  0,
		"net":  n.Network,
		"type": "none",
		"host": n.Host,
		"path": n.Path,
		"tls":  "",
	}
	if n.Network == "" {
		v["net"] = model.TransportTCP
	}
	if n.Security == model.SecurityTLS {
		v["tls"] = "tls"
		v["sni"] = n.ServerName
		if len(n.ALPN) > 0 {
			v["alpn"] = strings.Join(n.ALPN, ",")
		}
		if n.Fingerprint != "" {
			v["fp"] = n.Fingerprint
		}
	}

	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}

func emitWireGuard(n *model.ProxyNode) (string, error) {
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
	v := url.Values{}
	if n.PrivateKey != "" {
		v.Set("privatekey", n.PrivateKey)
	}
	if n.PeerPubKey != "" {
		v.Set("publickey", n.PeerPubKey)
	}
	if len(n.LocalAddr) > 0 {
		v.Set("ip", strings.Join(n.LocalAddr, ","))
	}
	if n.MTU > 0 {
		v.Set("mtu", strconv.Itoa(n.MTU))
	}
	if n.PreSharedKey != "" {
		v.Set("presharedkey", n.PreSharedKey)
	}

	uri := fmt.Sprintf("wireguard://%s", hostPort)
	q := v.Encode()
	if q != "" {
		uri += "?" + q
	}
	if n.Tag != "" {
		uri += "#" + url.QueryEscape(n.Tag)
	}
	return uri, nil
}
