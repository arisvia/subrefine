package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/api"
	"github.com/arisvia/subrefine/internal/config"
	"github.com/arisvia/subrefine/internal/storage"
)

func newAuthServer(t *testing.T) (http.Handler, *mockSubManager) {
	t.Helper()
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("init storage: %v", err)
	}
	mock := &mockSubManager{expectedID: "sub1", expectedToken: "tok123"}
	cfg := &config.Config{Host: "127.0.0.1", Port: 3000, AdminToken: "my_admin_secret"}
	return api.NewServer(cfg, mock, store).Handler(), mock
}

func adminReq(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/subscriptions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer my_admin_secret")
	return req
}

// The admin tier guards a single shared bearer token, so repeated failures must
// stop being free before an unattended script can grind through candidates.
func TestAdminAuthFailuresAreThrottled(t *testing.T) {
	handler, _ := newAuthServer(t)

	for i := range 20 {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 after the failure budget", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After so the console can back off")
	}

	// The throttle must not outlive the credential: a correct token still works
	// for a *different* client address.
	other := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
	other.Header.Set("Authorization", "Bearer my_admin_secret")
	other.RemoteAddr = "198.51.100.7:4444"
	otherRec := httptest.NewRecorder()
	handler.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusOK {
		t.Fatalf("other client status = %d, want 200 (throttle is per address)", otherRec.Code)
	}
}

// A successful login resets the budget, so an operator who mistypes repeatedly
// is never locked out by their own earlier mistakes once they get it right.
func TestSuccessfulAuthClearsFailureBudget(t *testing.T) {
	handler, _ := newAuthServer(t)

	for range 19 {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	ok := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
	ok.Header.Set("Authorization", "Bearer my_admin_secret")
	handler.ServeHTTP(httptest.NewRecorder(), ok)

	for range 19 {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("after a success, status = %d, want 401", rec.Code)
		}
	}
}

// Configuration the delivery engine would silently ignore must be refused at the
// save boundary, because the console makes a typo one keystroke away.
func TestAdminSaveRejectsUnusableConfig(t *testing.T) {
	handler, _ := newAuthServer(t)

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{
			name:    "empty id",
			body:    `{"id":"","sources":["ss://a@b:1"]}`,
			wantMsg: "id is required",
		},
		{
			name:    "id with separator",
			body:    `{"id":"a/b","sources":["ss://a@b:1"]}`,
			wantMsg: "must match",
		},
		{
			name:    "unknown operator",
			body:    `{"id":"ok","sources":["ss://a@b:1"],"pipeline":[{"type":"quantum"}]}`,
			wantMsg: "unknown operator",
		},
		{
			name:    "bad rename regex",
			body:    `{"id":"ok","sources":["ss://a@b:1"],"pipeline":[{"type":"rename","pattern":"([","repl":"x"}]}`,
			wantMsg: "invalid pattern",
		},
		{
			name:    "bad filter regex",
			body:    `{"id":"ok","sources":["ss://a@b:1"],"pipeline":[{"type":"filter","include":"*bad("}]}`,
			wantMsg: "invalid include pattern",
		},
		{
			name:    "no recognizable protocol",
			body:    `{"id":"ok","sources":["ss://a@b:1"],"pipeline":[{"type":"filter","types":["nope","nada"]}]}`,
			wantMsg: "no recognizable protocol",
		},
		{
			name:    "bad sort key",
			body:    `{"id":"ok","sources":["ss://a@b:1"],"pipeline":[{"type":"sort","by":"latency"}]}`,
			wantMsg: "unknown sort key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, adminReq(tc.body))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("error body is not JSON: %v (%s)", err, rec.Body.String())
			}
			if !strings.Contains(payload["error"], tc.wantMsg) {
				t.Fatalf("error = %q, want it to mention %q", payload["error"], tc.wantMsg)
			}
		})
	}

	// A partially recognizable allow-list is still accepted, and a valid body saves.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, adminReq(`{"id":"ok","name":"OK","token":"t","sources":["ss://a@b:1"],"pipeline":[{"type":"filter","types":["ss","bogus"]},{"type":"sort","by":"tag","order":"desc"}]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid save: status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// A disabled subscription is expected in stored data (e2e off1, the enable
// toggle); its preview is a client-visible conflict, not a 500.
func TestAdminPreviewOfDisabledSubIs409(t *testing.T) {
	handler, _ := newAuthServer(t)

	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, adminReq(`{"id":"off","token":"t","sources":["ss://a@b:1"],"enabled":false}`))
	if seed.Code != http.StatusOK {
		t.Fatalf("seed save: status = %d (%s)", seed.Code, seed.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions/off/nodes", nil)
	req.Header.Set("Authorization", "Bearer my_admin_secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("disabled preview: status = %d, want 409 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("body = %s, want it to say why", rec.Body.String())
	}
}

func TestAdminDeleteInvalidatesDeliveryCache(t *testing.T) {
	handler, mock := newAuthServer(t)

	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, adminReq(`{"id":"bye","token":"t","sources":["ss://a@b:1"]}`))
	if seed.Code != http.StatusOK {
		t.Fatalf("seed save: status = %d", seed.Code)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/subscriptions/bye", nil)
	req.Header.Set("Authorization", "Bearer my_admin_secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete: status = %d, want 200", rec.Code)
	}
	if len(mock.invalidated) != 1 || mock.invalidated[0] != "bye" {
		t.Fatalf("invalidated = %v, want [bye] so cached nodes are released", mock.invalidated)
	}
}

// The console keeps the admin bearer token in localStorage, so shipped scripts
// must stay same-origin-only for that choice to be safe.
func TestDashboardSendsHardeningHeaders(t *testing.T) {
	handler, _ := newAuthServer(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "font-src 'self' data:"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP %q missing %q", csp, want)
		}
	}
	if strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("CSP %q must not allow eval", csp)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// A mistyped API path must not be answered with the console shell: airport
// clients would cache that HTML as subscription content.
func TestUnknownAPIPathIsNotTheShell(t *testing.T) {
	handler, _ := newAuthServer(t)

	for _, path := range []string{"/api/admin/subscriptions", "/api/admin/typo", "/subscriptions"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))

		if rec.Code == http.StatusOK && strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("POST %s returned the console shell: %d %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
