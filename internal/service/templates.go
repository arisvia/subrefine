package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/arisvia/subrefine/internal/core/emitter"
)

// Templates is the startup-loaded registry of full-config templates, keyed by
// mode name (tun, tproxy, mixed, ...). File naming is the contract:
//
//	<dir>/singbox.<mode>.json   official sing-box skeleton
//	<dir>/clash.<mode>.yaml     mihomo skeleton (.yml accepted)
//
// A broken template fails the process at startup: an operator must never
// discover at delivery time that the "tun" mode was silently unloadable.
type Templates struct {
	Singbox map[string]*emitter.SingboxTemplate
	Clash   map[string]*emitter.ClashFullTemplate
}

var (
	singboxNameRE = regexp.MustCompile(`^singbox\.(.+)\.json$`)
	clashNameRE   = regexp.MustCompile(`^clash\.(.+)\.ya?ml$`)
)

// LoadTemplates scans dir; an empty dir means the feature is off (nil, nil).
func LoadTemplates(dir string) (*Templates, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("templates dir: %w", err)
	}

	t := &Templates{
		Singbox: map[string]*emitter.SingboxTemplate{},
		Clash:   map[string]*emitter.ClashFullTemplate{},
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(dir, name)
		if m := singboxNameRE.FindStringSubmatch(name); m != nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			tmpl, err := emitter.LoadSingboxTemplate(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			t.Singbox[m[1]] = tmpl
		} else if m := clashNameRE.FindStringSubmatch(name); m != nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			tmpl, err := emitter.LoadClashFullTemplate(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			t.Clash[m[1]] = tmpl
		}
	}
	return t, nil
}

// Modes lists the registered mode names, for the admin endpoint and console.
func (t *Templates) Modes() (singbox, clash []string) {
	if t == nil {
		return nil, nil
	}
	for k := range t.Singbox {
		singbox = append(singbox, k)
	}
	for k := range t.Clash {
		clash = append(clash, k)
	}
	sort.Strings(singbox)
	sort.Strings(clash)
	return
}

// SingboxTemplate returns the named full-config template, nil when absent.
func (t *Templates) SingboxTemplate(mode string) *emitter.SingboxTemplate {
	if t == nil {
		return nil
	}
	return t.Singbox[mode]
}

// ClashTemplate returns the named full-config template, nil when absent.
func (t *Templates) ClashTemplate(mode string) *emitter.ClashFullTemplate {
	if t == nil {
		return nil
	}
	return t.Clash[mode]
}
