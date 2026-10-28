package api

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arisvia/subrefine/webui"
)

// DashboardHandler serves the embedded console with two tiers:
//  1. A local directory (dev override) so `npm run build` output can be served
//     without recompiling the Go binary.
//  2. The webui/dist tree embedded at build time (production).
//
// Vite emits content-hashed assets plus .br/.gz sidecars, so responses are
// negotiated against Accept-Encoding (RFC 9110 qvalues) and carry the matching
// cache policy: hashed assets immutable, index.html never cached.
type DashboardHandler struct {
	devDir   string
	embedded fs.FS
}

func NewDashboardHandler(devDir string) *DashboardHandler {
	return newDashboardHandler(devDir, webui.Dist())
}

// newDashboardHandler binds a specific console filesystem, letting tests drive
// both tiers without rebuilding the binary.
func newDashboardHandler(devDir string, embedded fs.FS) *DashboardHandler {
	return &DashboardHandler{devDir: devDir, embedded: embedded}
}

func (d *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addDashboardVary(w.Header())
	setDashboardSecurity(w.Header())
	name := dashboardName(r)

	if d.devDir != "" && serveDashboardFile(w, r, os.DirFS(d.devDir), name) {
		return
	}
	if d.embedded != nil && serveDashboardFile(w, r, d.embedded, name) {
		return
	}
	// Nothing matched. An absent shell means the console was never built;
	// otherwise the path is unknown and must not receive a shell page.
	if d.hasIndex() {
		http.NotFound(w, r)
		return
	}
	http.Error(w, "console assets are missing; build webui/dist before compiling", http.StatusServiceUnavailable)
}

// dashboardName maps a request to the dist-relative file it wants; the bare
// root and /index.html both resolve to the console shell.
func dashboardName(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "index.html" || strings.HasSuffix(path, "/index.html") {
		return "index.html"
	}
	return path
}

// hasIndex reports whether any tier carries the console shell.
func (d *DashboardHandler) hasIndex() bool {
	if d.devDir != "" {
		if _, err := os.Stat(filepath.Join(d.devDir, "index.html")); err == nil {
			return true
		}
	}
	if d.embedded == nil {
		return false
	}
	_, err := fs.Stat(d.embedded, "index.html")
	return err == nil
}

func serveDashboardFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	// Validate before using os.DirFS, including Windows path separators.
	if !fs.ValidPath(name) || strings.Contains(name, `\`) {
		return false
	}
	info, err := fs.Stat(fsys, name)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}

	var file http.File
	selected, encoding := false, ""
	for _, coding := range dashboardEncodings(r.Header) {
		if coding == "identity" {
			selected = true
			break
		}
		suffix := ".br"
		if coding == "gzip" {
			suffix = ".gz"
		}
		f, err := http.FS(fsys).Open(name + suffix)
		if err != nil {
			continue
		}
		stat, err := f.Stat()
		if err != nil || !stat.Mode().IsRegular() {
			f.Close()
			continue
		}
		file, info, selected, encoding = f, stat, true, coding
		break
	}
	if !selected {
		http.Error(w, "no acceptable content encoding", http.StatusNotAcceptable)
		return true
	}

	// Open index entries directly to avoid redirects and allow tier fallback.
	isIndex := filepath.Base(name) == "index.html"
	if file == nil && (isIndex || strings.HasSuffix(r.URL.Path, "/index.html")) {
		file, err = http.FS(fsys).Open(name)
		if err != nil {
			return false
		}
	}
	if file != nil {
		defer file.Close()
	}
	if encoding != "" {
		// Sniff the original, never the compressed bytes, for unknown extensions.
		contentType := mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			f, err := fsys.Open(name)
			if err != nil {
				return false
			}
			var buf [512]byte
			n, _ := io.ReadFull(f, buf[:])
			f.Close()
			contentType = http.DetectContentType(buf[:n])
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Encoding", encoding)
	}
	if isIndex {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	// Reuse the open sidecar/entry without reopening or buffering its contents.
	if file != nil {
		http.ServeContent(w, r, name, info.ModTime(), file)
	} else {
		http.ServeFileFS(w, r, fsys, name)
	}
	return true
}

func addDashboardVary(h http.Header) {
	for _, line := range h.Values("Vary") {
		for token := range strings.SplitSeq(line, ",") {
			token = strings.TrimSpace(token)
			if token == "*" || strings.EqualFold(token, "Accept-Encoding") {
				return
			}
		}
	}
	h.Add("Vary", "Accept-Encoding")
}

// setDashboardSecurity pins the console to its own scripts. The admin bearer
// token is kept in localStorage, so an injected script on this origin would
// read it; 'self' without unsafe-inline or unsafe-eval is what makes that
// storage choice defensible. Only two relaxations remain, each with a live
// justification: style 'unsafe-inline' because the UI sets CSS custom
// properties per element, and nothing else leaves the same origin - the
// bundle contains no data:/blob: font or image URLs (the CJK @fontsource
// package that inlined data: subsets was removed from the payload).
func setDashboardSecurity(h http.Header) {
	if h.Get("Content-Security-Policy") == "" {
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	}
	if h.Get("X-Content-Type-Options") == "" {
		h.Set("X-Content-Type-Options", "nosniff")
	}
	if h.Get("Referrer-Policy") == "" {
		h.Set("Referrer-Policy", "same-origin")
	}
}

// Unspecified identity is a last resort unless *;q=0 excludes it.
func dashboardEncodings(h http.Header) []string {
	weights := make(map[string]int, 4)
	for _, line := range h.Values("Accept-Encoding") {
		for item := range strings.SplitSeq(line, ",") {
			token, params, hasParams := strings.Cut(item, ";")
			token = strings.ToLower(strings.TrimSpace(token))
			switch token {
			case "br", "gzip", "identity", "*":
			default:
				continue
			}
			q := 1000
			if hasParams {
				key, value, ok := strings.Cut(params, "=")
				q = 0
				if ok && strings.EqualFold(strings.TrimSpace(key), "q") {
					q = dashboardEncodingQuality(strings.TrimSpace(value))
				}
			}
			// Conflicting duplicates must not resurrect an explicit prohibition.
			if previous, ok := weights[token]; ok {
				q = min(q, previous)
			}
			weights[token] = q
		}
	}
	var accepted []string
	for _, coding := range []string{"br", "gzip", "identity"} {
		q, explicit := weights[coding]
		if !explicit {
			wildcard, present := weights["*"]
			q = wildcard
			if coding == "identity" && (!present || wildcard != 0) {
				q = -1 // Allowed, but no explicit preference over compression.
			}
		}
		weights[coding] = q
		if q != 0 {
			accepted = append(accepted, coding)
		}
	}
	slices.SortStableFunc(accepted, func(a, b string) int { return weights[b] - weights[a] })
	return accepted
}

// Parse RFC 9110 qvalues; invalid weights disable the coding.
func dashboardEncodingQuality(value string) int {
	whole, fraction, _ := strings.Cut(value, ".")
	if (whole != "0" && whole != "1") || len(fraction) > 3 {
		return 0
	}
	q, place := 0, 100
	for _, digit := range fraction {
		if digit < '0' || digit > '9' || (whole == "1" && digit != '0') {
			return 0
		}
		q += int(digit-'0') * place
		place /= 10
	}
	if whole == "1" {
		return 1000
	}
	return q
}
