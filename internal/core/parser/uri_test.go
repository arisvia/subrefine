package parser_test

import (
	"testing"

	"subrefine/internal/core/emitter"
	"subrefine/internal/core/model"
	"subrefine/internal/core/parser"
)

func TestParseAndEmitURI(t *testing.T) {
	tests := []struct {
		name  string
		uri   string
		check func(t *testing.T, node *model.ProxyNode)
	}{
		{
			name: "Shadowsocks",
			uri:  "ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@1.2.3.4:8388#SSNode",
			check: func(t *testing.T, node *model.ProxyNode) {
				if node.Type != model.ProtoShadowsocks {
					t.Fatalf("expected ss, got %s", node.Type)
				}
				if node.Method != "aes-128-gcm" || node.Password != "password" {
					t.Fatalf("unexpected method or pass: %s:%s", node.Method, node.Password)
				}
				if node.Server != "1.2.3.4" || node.Port != 8388 {
					t.Fatalf("unexpected addr: %s:%d", node.Server, node.Port)
				}
				if node.Tag != "SSNode" {
					t.Fatalf("unexpected tag: %s", node.Tag)
				}
			},
		},
		{
			name: "VLESS Reality Vision",
			uri:  "vless://11111111-2222-3333-4444-555555555555@example.com:443?security=reality&flow=xtls-rprx-vision&sni=speed.cloudflare.com&fp=chrome&pbk=fakePubkey&sid=fakeSid#RealityNode",
			check: func(t *testing.T, node *model.ProxyNode) {
				if node.Type != model.ProtoVLESS {
					t.Fatalf("expected vless, got %s", node.Type)
				}
				if node.UUID != "11111111-2222-3333-4444-555555555555" {
					t.Fatalf("unexpected uuid: %s", node.UUID)
				}
				if node.Security != model.SecurityReality || node.Flow != "xtls-rprx-vision" {
					t.Fatalf("unexpected reality params: %s %s", node.Security, node.Flow)
				}
				if node.ServerName != "speed.cloudflare.com" || node.PublicKey != "fakePubkey" {
					t.Fatalf("unexpected sni/pbk: %s %s", node.ServerName, node.PublicKey)
				}
			},
		},
		{
			name: "Hysteria2",
			uri:  "hy2://pass123@my.server.org:8443?sni=my.server.org&insecure=1&obfs=salamander&obfs-password=secret#Hy2Node",
			check: func(t *testing.T, node *model.ProxyNode) {
				if node.Type != model.ProtoHysteria2 {
					t.Fatalf("expected hysteria2, got %s", node.Type)
				}
				if node.Password != "pass123" {
					t.Fatalf("unexpected pass: %s", node.Password)
				}
				if !node.Insecure || node.ObfsType != "salamander" || node.ObfsPass != "secret" {
					t.Fatalf("unexpected hy2 options")
				}
			},
		},
		{
			name: "Trojan WS",
			uri:  "trojan://mypass@trojan.com:443?type=ws&host=trojan.com&path=%2Fws#TrojanWS",
			check: func(t *testing.T, node *model.ProxyNode) {
				if node.Type != model.ProtoTrojan {
					t.Fatalf("expected trojan, got %s", node.Type)
				}
				if node.Password != "mypass" {
					t.Fatalf("unexpected pass: %s", node.Password)
				}
				if node.Network != "ws" || node.Path != "/ws" {
					t.Fatalf("unexpected transport: %s %s", node.Network, node.Path)
				}
			},
		},
		{
			name: "WireGuard",
			uri:  "wireguard://1.2.3.4:51820?ip=10.0.0.2%2F32&mtu=1420&presharedkey=psk123&privatekey=privkey123&publickey=pubkey123#WGNode",
			check: func(t *testing.T, node *model.ProxyNode) {
				if node.Type != model.ProtoWireGuard {
					t.Fatalf("expected wireguard, got %s", node.Type)
				}
				if node.PrivateKey != "privkey123" || node.PeerPubKey != "pubkey123" {
					t.Fatalf("unexpected keys")
				}
				if node.MTU != 1420 || len(node.LocalAddr) == 0 || node.LocalAddr[0] != "10.0.0.2/32" {
					t.Fatalf("unexpected wg params")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parser.ParseURI(tt.uri)
			if err != nil {
				t.Fatalf("ParseURI error: %v", err)
			}
			tt.check(t, node)

			emitted, err := emitter.ToURI(node)
			if err != nil {
				t.Fatalf("ToURI error: %v", err)
			}

			// Re-parse emitted URI to guarantee round-trip fidelity
			reparsed, err := parser.ParseURI(emitted)
			if err != nil {
				t.Fatalf("Re-ParseURI error: %v (emitted: %s)", err, emitted)
			}
			tt.check(t, reparsed)
		})
	}
}
