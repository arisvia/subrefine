package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// dashboardFixture builds a two-tier tree: an optional local directory (dev
// override) and an embedded fs holding index.html plus one hashed asset and its
// precompressed sidecars.
type dashboardFixture struct {
	localDir string
	embedded fs.FS
}

func (f dashboardFixture) handler() http.Handler {
	return newDashboardHandler(f.localDir, f.embedded)
}

func newDashboardFixture(t *testing.T) dashboardFixture {
	t.Helper()
	dir := t.TempDir()
	writeFile := func(name, data string) {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("index.html", "<!doctype html><title>local console</title>")
	writeFile("assets/index-abc123.js", "local javascript")
	writeFile("assets/index-abc123.js.br", "local brotli")
	writeFile("assets/index-abc123.js.gz", "local gzip")
	return dashboardFixture{
		localDir: dir,
		embedded: fstest.MapFS{
			"index.html":                 {Data: []byte("<!doctype html><title>embedded console</title>")},
			"assets/index-abc123.js":     {Data: []byte("embedded javascript")},
			"assets/index-abc123.js.br":  {Data: []byte("embedded brotli")},
			"assets/index-abc123.js.gz":  {Data: []byte("embedded gzip")},
			"assets/index-abc123.css":    {Data: []byte("embedded css")},
			"assets/index-abc123.css.gz": {Data: []byte("embedded css gzip")},
		},
	}
}

func TestDashboardHandlerNegotiatesPrecompressedAssets(t *testing.T) {
	fixture := newDashboardFixture(t)
	tests := []struct {
		name           string
		acceptEncoding string
		wantBody       string
		wantEncoding   string
		wantStatus     int
	}{
		{name: "brotli", acceptEncoding: "br, gzip", wantBody: "local brotli", wantEncoding: "br", wantStatus: http.StatusOK},
		{name: "gzip only", acceptEncoding: "gzip", wantBody: "local gzip", wantEncoding: "gzip", wantStatus: http.StatusOK},
		{name: "identity only", acceptEncoding: "identity", wantBody: "local javascript", wantEncoding: "", wantStatus: http.StatusOK},
		{name: "qvalue order", acceptEncoding: "gzip;q=0.8, br;q=0.5", wantBody: "local gzip", wantEncoding: "gzip", wantStatus: http.StatusOK},
		{name: "brotli prohibited", acceptEncoding: "br;q=0, gzip", wantBody: "local gzip", wantEncoding: "gzip", wantStatus: http.StatusOK},
		{name: "compression prohibited", acceptEncoding: "br;q=0, gzip;q=0, identity;q=1", wantBody: "local javascript", wantEncoding: "", wantStatus: http.StatusOK},
		{name: "unspecified identity prohibited", acceptEncoding: "gzip, *;q=0", wantBody: "local gzip", wantEncoding: "gzip", wantStatus: http.StatusOK},
		{name: "all prohibited", acceptEncoding: "br;q=0, gzip;q=0, identity;q=0", wantBody: "no acceptable content encoding\n", wantEncoding: "", wantStatus: http.StatusNotAcceptable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()

			fixture.handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Fatalf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
				t.Fatalf("Cache-Control = %q, want immutable asset policy", got)
			}
		})
	}
}

func TestDashboardHandlerDetectsContentTypeForUnknownSidecar(t *testing.T) {
	fixture := dashboardFixture{
		embedded: fstest.MapFS{
			"index.html":     {Data: []byte("<!doctype html><title>console</title>")},
			"weird":          {Data: []byte("plain text")},
			"weird.br":       {Data: []byte("brotli")},
			"unknown":        {Data: []byte("plain text")},
			"unknown.gz":     {Data: []byte("gzip")},
			"unknownjs.js.x": {Data: []byte("not javascript")},
		},
	}

	tests := []struct {
		path           string
		acceptEncoding string
		want           string
	}{
		{path: "/weird", acceptEncoding: "identity", want: "text/plain; charset=utf-8"},
		// No extension to map, so the type comes from sniffing the original file.
		{path: "/unknown", acceptEncoding: "gzip", want: "text/plain; charset=utf-8"},
		{path: "/unknownjs.js.x", acceptEncoding: "identity", want: "text/plain; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			rec := httptest.NewRecorder()
			fixture.handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.want {
				t.Fatalf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDashboardHandlerServesSPAWithoutRedirect(t *testing.T) {
	fixture := newDashboardFixture(t)
	tests := []struct {
		name string
		path string
	}{
		{name: "index", path: "/"},
		{name: "explicit index", path: "/index.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "br, gzip")
			rec := httptest.NewRecorder()

			fixture.handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Body.String(); !strings.Contains(got, "local console") {
				t.Fatalf("body = %q, want local console", got)
			}
			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("Content-Encoding = %q, want none for an uncompressed shell", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
				t.Fatalf("Cache-Control = %q, want no-store for index", got)
			}
		})
	}

	// Routing lives in the URL fragment, which browsers never send, so a deep
	// path must not quietly receive the shell: that hides typos as a blank page.
	t.Run("deep path is not masked by the shell", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
		rec := httptest.NewRecorder()

		fixture.handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestDashboardHandlerFallsBackToEmbeddedWhenLocalDirUnavailable(t *testing.T) {
	fixture := newDashboardFixture(t)
	if err := os.RemoveAll(fixture.localDir); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.css", nil)
	req.Header.Set("Accept-Encoding", "identity")
	rec := httptest.NewRecorder()

	fixture.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "embedded css" {
		t.Fatalf("embedded fallback failed: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestDashboardHandlerReportsMissingAssetsInsteadOfSPA(t *testing.T) {
	fixture := newDashboardFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()

	fixture.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Body.String(); strings.Contains(got, "console") {
		t.Fatalf("body = %q, want a 404 rather than the console shell", got)
	}
}

func TestDashboardHandlerRejectsTraversalAndPreservesVary(t *testing.T) {
	fixture := newDashboardFixture(t)
	tests := []struct {
		name    string
		path    string
		handler http.Handler
	}{
		{name: "parent traversal", path: "/../secret.txt", handler: fixture.handler()},
		{name: "encoded traversal", path: "/%2e%2e%2fsecret.txt", handler: fixture.handler()},
		{name: "windows traversal", path: `/..\secret.txt`, handler: fixture.handler()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if got := rec.Body.String(); strings.Contains(got, "secret") || strings.Contains(got, "console") {
				t.Fatalf("traversal served: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("explicit Vary is not duplicated", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Accept-Encoding")
			NewDashboardHandler("").ServeHTTP(w, r)
		})
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if got := rec.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept-Encoding" {
			t.Fatalf("Vary = %#v, want one Accept-Encoding", got)
		}
	})
}

func TestDashboardHandlerFallsBackWhenLocalAssetMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>local console</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := dashboardFixture{
		localDir: dir,
		embedded: fstest.MapFS{
			"index.html":              {Data: []byte("<!doctype html><title>embedded console</title>")},
			"assets/missing.css":      {Data: []byte("embedded css")},
			"assets/missing.css.br":   {Data: []byte("embedded brotli")},
			"assets/missing.css.gz":   {Data: []byte("embedded gzip")},
			"assets/uncompressed.css": {Data: []byte("embedded uncompressed css")},
		},
	}

	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		wantBody       string
		wantEncoding   string
	}{
		{name: "compressed asset", path: "/assets/missing.css", acceptEncoding: "br, gzip", wantBody: "embedded brotli", wantEncoding: "br"},
		{name: "uncompressed asset", path: "/assets/uncompressed.css", acceptEncoding: "identity", wantBody: "embedded uncompressed css"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()

			fixture.handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Fatalf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}
			if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
				t.Fatalf("Cache-Control = %q, want immutable asset policy", got)
			}
		})
	}
}
