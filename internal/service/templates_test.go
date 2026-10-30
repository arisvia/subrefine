package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/service"
)

func writeTemplate(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

const sbTunMixed = `{"inbounds":[{"type":"mixed","tag":"m","listen":"127.0.0.1","listen_port":7890},{"type":"tun","tag":"t","auto_route":true}],"outbounds":[{"type":"direct","tag":"@subrefine"}]}`
const clTun = "mixed-port: 7890\ntun:\n  enable: true\nproxies:\n  - {name: \"@subrefine\"}\n"

func TestLoadTemplatesSingleFilePerFamily(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "singbox.json", sbTunMixed)
	writeTemplate(t, dir, "clash.yaml", clTun)
	// Legacy per-mode files and stray files must now be ignored.
	writeTemplate(t, dir, "singbox.tun.json", `{"outbounds":[]}`)
	writeTemplate(t, dir, "notes.txt", "ignored")

	templates, err := service.LoadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	sb, cl := templates.Options()
	if !sb.Available || strings.Join(sb.Inbounds, ",") != "mixed,tun" {
		t.Fatalf("singbox options: %+v", sb)
	}
	if !cl.Available || strings.Join(cl.Inbounds, ",") != "mixed,tun" {
		t.Fatalf("clash options: %+v", cl)
	}
	if templates.SingboxTemplate() == nil || templates.ClashTemplate() == nil {
		t.Fatal("accessors must return the loaded templates")
	}
	// The broken legacy singbox.tun.json was ignored (a real singbox.json
	// loaded fine, so startup did not abort on the legacy file).
	if templates.Empty() {
		t.Fatal("registry should not be empty")
	}
}

func TestLoadTemplatesFailsFast(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "singbox.json", `{"outbounds":[]}`) // no injection point
	if _, err := service.LoadTemplates(dir); err == nil {
		t.Fatal("broken template must abort startup")
	}
}

func TestLoadTemplatesPartialFamily(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "clash.yml", clTun) // .yml accepted
	templates, err := service.LoadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	sb, cl := templates.Options()
	if sb.Available {
		t.Fatal("singbox should be absent")
	}
	if !cl.Available {
		t.Fatal("clash (.yml) should load")
	}
}

func TestLoadTemplatesEmptyDirOff(t *testing.T) {
	templates, err := service.LoadTemplates("")
	if err != nil || templates != nil {
		t.Fatalf("empty dir must mean off: %v %v", templates, err)
	}
	sb, cl := templates.Options() // nil receiver safety
	if sb.Available || cl.Available || sb.Inbounds == nil || cl.Inbounds == nil {
		t.Fatalf("nil Options must be empty non-nil: %+v %+v", sb, cl)
	}
	if templates.SingboxTemplate() != nil || templates.ClashTemplate() != nil {
		t.Fatal("nil accessors must be nil")
	}
	if !templates.Empty() {
		t.Fatal("nil registry must report Empty")
	}
}
