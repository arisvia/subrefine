package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arisvia/subrefine/internal/core/emitter"
)

// Templates is the startup-loaded registry of full-config templates. There is
// exactly ONE generic template per client family; the per-delivery options
// (which inbounds, IPv6) are chosen at request time and baked into the link,
// so a single template serves every combination. File naming is the contract:
//
//	<dir>/singbox.json   official sing-box skeleton
//	<dir>/clash.yaml     mihomo skeleton (.yml accepted)
//
// A broken template fails the process at startup: an operator must never
// discover at delivery time that the full-config path was silently unloadable.
type Templates struct {
	Singbox *emitter.SingboxTemplate
	Clash   *emitter.ClashFullTemplate
}

// LoadTemplates reads the fixed template files from dir. An empty dir means
// the feature is off (nil, nil); a missing individual file just means that
// family has no full-config template.
func LoadTemplates(dir string) (*Templates, error) {
	if dir == "" {
		return nil, nil
	}
	t := &Templates{}

	if data, ok, err := readTemplateFile(dir, "singbox.json"); err != nil {
		return nil, err
	} else if ok {
		tmpl, err := emitter.LoadSingboxTemplate(data)
		if err != nil {
			return nil, fmt.Errorf("singbox.json: %w", err)
		}
		t.Singbox = tmpl
	}

	for _, name := range []string{"clash.yaml", "clash.yml"} {
		data, ok, err := readTemplateFile(dir, name)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		tmpl, err := emitter.LoadClashFullTemplate(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		t.Clash = tmpl
		break
	}

	return t, nil
}

func readTemplateFile(dir, name string) ([]byte, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", name, err)
	}
	return data, true, nil
}

// Empty reports whether no full-config template was loaded at all.
func (t *Templates) Empty() bool { return t == nil || (t.Singbox == nil && t.Clash == nil) }

// Options reports the declared inbound kinds per family so the console renders
// only switches that mean something for the loaded templates.
func (t *Templates) Options() (singbox, clash emitter.FamilyOptions) {
	singbox, clash = emitter.FamilyOptions{Inbounds: []string{}}, emitter.FamilyOptions{Inbounds: []string{}}
	if t == nil {
		return
	}
	if t.Singbox != nil {
		singbox.Available = true
		if ins := t.Singbox.InboundOptions(); ins != nil {
			singbox.Inbounds = ins
		}
	}
	if t.Clash != nil {
		clash.Available = true
		if ins := t.Clash.InboundOptions(); ins != nil {
			clash.Inbounds = ins
		}
	}
	return
}

// SingboxTemplate returns the sing-box full-config template, nil when absent.
func (t *Templates) SingboxTemplate() *emitter.SingboxTemplate {
	if t == nil {
		return nil
	}
	return t.Singbox
}

// ClashTemplate returns the clash full-config template, nil when absent.
func (t *Templates) ClashTemplate() *emitter.ClashFullTemplate {
	if t == nil {
		return nil
	}
	return t.Clash
}
