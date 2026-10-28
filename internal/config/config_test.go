package config

import (
	"os"
	"testing"
)

// A published binary must never come up protected by a credential that is
// readable in the repository.
func TestDefaultConfigRefusesToStart(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.AdminToken != "" {
		t.Fatalf("default admin token = %q, want empty so startup is refused", cfg.AdminToken)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error when no admin token is set")
	}
}

func TestAdminTokenComesFromEnvironment(t *testing.T) {
	t.Setenv("SUBREFINE_ADMIN_TOKEN", "operator-secret")
	t.Setenv("SUBREFINE_PORT", "8080")

	cfg := DefaultConfig()
	LoadFromEnv(cfg)

	if cfg.AdminToken != "operator-secret" {
		t.Fatalf("AdminToken = %q, want the environment value", cfg.AdminToken)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil once a token is supplied", err)
	}
}

// An empty override must not resurrect a credential, and must not silently
// discard a malformed one either.
func TestLoadFromEnvIgnoresEmptyAndInvalid(t *testing.T) {
	os.Clearenv()
	cfg := DefaultConfig()
	cfg.AdminToken = "already-set"
	cfg.Port = 3000

	t.Setenv("SUBREFINE_ADMIN_TOKEN", "")
	t.Setenv("SUBREFINE_PORT", "not-a-port")
	LoadFromEnv(cfg)

	if cfg.AdminToken != "already-set" {
		t.Fatalf("AdminToken = %q, want the previous value kept", cfg.AdminToken)
	}
	if cfg.Port != 3000 {
		t.Fatalf("Port = %d, want the previous value kept for an unparseable port", cfg.Port)
	}
}
