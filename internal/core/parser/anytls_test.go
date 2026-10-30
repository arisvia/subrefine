package parser

import (
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/core/emitter"
	"github.com/arisvia/subrefine/internal/core/model"
)

func TestAnyTLSRoundTrip(t *testing.T) {
	uris := []string{
		`anytls://ca7b0f4a-87d7-4776-a36e-eac124a75fc9@[2607:f5b4:88:106:1c00:ccff:fe00:40b]:9449?sni=dfgdghh.uasese.baby&insecure=0#%5Banytls%5D%F0%9F%87%AD`,
		`anytls://ca7b0f4a-87d7-4776-a36e-eac124a75fc9@103.6.235.28:9443?sni=kTWZgp.vnanan.cyou&insecure=1#%F0%9F%87%BBViet`,
	}
	for _, raw := range uris {
		n, err := ParseURI(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if n.Type != model.ProtoAnyTLS {
			t.Fatalf("type = %s, want anytls", n.Type)
		}
		if n.Password != "ca7b0f4a-87d7-4776-a36e-eac124a75fc9" {
			t.Errorf("password = %q", n.Password)
		}
		if n.ServerName == "" {
			t.Errorf("sni lost")
		}
		// sing-box outbound
		sb, err := emitter.ToSingboxOutbound(n)
		if err != nil {
			t.Fatalf("singbox: %v", err)
		}
		if sb["type"] != "anytls" || sb["password"] == nil {
			t.Errorf("singbox bad: %v", sb["type"])
		}
		tls, _ := sb["tls"].(map[string]any)
		if tls == nil || tls["enabled"] != true {
			t.Errorf("singbox tls bad: %v", sb["tls"])
		}
		if strings.Contains(raw, "insecure=1") != (tls["insecure"] == true) {
			t.Errorf("insecure mapping wrong: raw=%s tls=%v", raw, tls["insecure"])
		}
		// clash proxy
		cl, err := emitter.ToClashProxy(n)
		if err != nil {
			t.Fatalf("clash: %v", err)
		}
		if cl["type"] != "anytls" || cl["password"] == nil {
			t.Errorf("clash bad: %v", cl["type"])
		}
		// uri round-trip
		u, err := emitter.ToURI(n)
		if err != nil {
			t.Fatalf("uri: %v", err)
		}
		if !strings.HasPrefix(u, "anytls://") {
			t.Errorf("uri scheme bad: %s", u)
		}
		n2, err := ParseURI(u)
		if err != nil || n2.Server != n.Server || n2.Port != n.Port || n2.Password != n.Password {
			t.Errorf("uri round-trip lost data: %s -> %v %v", u, n2, err)
		}
	}
}
