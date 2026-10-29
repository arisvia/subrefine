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

const okSB = `{"outbounds":[{"type":"direct","tag":"@subrefine"}]}`
const okCL = "proxies:\n  - {name: \"@subrefine\"}\n"

func TestLoadTemplatesNaming(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "singbox.tun.json", okSB)
	writeTemplate(t, dir, "singbox.tproxy.json", okSB)
	writeTemplate(t, dir, "clash.mixed.yaml", okCL)
	writeTemplate(t, dir, "clash.tun.yml", okCL)
	writeTemplate(t, dir, "notes.txt", "ignored")
	writeTemplate(t, dir, "singbox.json", "not a mode file")

	templates, err := service.LoadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	sb, cl := templates.Modes()
	if strings.Join(sb, ",") != "tproxy,tun" {
		t.Fatalf("singbox modes: %v", sb)
	}
	if strings.Join(cl, ",") != "mixed,tun" {
		t.Fatalf("clash modes: %v", cl)
	}
	if templates.SingboxTemplate("tun") == nil || templates.ClashTemplate("yml-does-not-exist") != nil {
		t.Fatal("accessor mismatch")
	}
}

func TestLoadTemplatesFailsFast(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "singbox.broken.json", `{"outbounds":[]}`) // no injection point
	if _, err := service.LoadTemplates(dir); err == nil {
		t.Fatal("broken template must abort startup")
	}
}

func TestLoadTemplatesEmptyDirOff(t *testing.T) {
	templates, err := service.LoadTemplates("")
	if err != nil || templates != nil {
		t.Fatalf("empty dir must mean off: %v %v", templates, err)
	}
	sb, cl := templates.Modes() // nil receiver safety
	if sb != nil || cl != nil {
		t.Fatal("nil Modes must be empty")
	}
	if templates.SingboxTemplate("tun") != nil || templates.ClashTemplate("tun") != nil {
		t.Fatal("nil accessors must be nil")
	}
}
