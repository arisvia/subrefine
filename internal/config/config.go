package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

type Config struct {
	Host         string `json:"host" yaml:"host"`
	Port         int    `json:"port" yaml:"port"`
	AdminToken   string `json:"admin_token" yaml:"admin_token"`
	DataDir      string `json:"data_dir" yaml:"data_dir"`
	DashboardDir string `json:"dashboard_dir" yaml:"dashboard_dir"`
	// TrustProxy makes the auth limiter key off X-Forwarded-For. Only enable it
	// when a proxy you control rewrites that header, otherwise any client can
	// pick its own throttle bucket by sending a header.
	TrustProxy bool `json:"trust_proxy" yaml:"trust_proxy"`
	// TemplatesDir holds singbox.<mode>.json / clash.<mode>.yaml full-config
	// templates; empty disables full-config delivery entirely.
	TemplatesDir string `json:"templates_dir" yaml:"templates_dir"`
}

func DefaultConfig() *Config {
	return &Config{
		Host: "0.0.0.0",
		Port: 3000,
		// No token ships with the binary: an operator must supply one, so a
		// deployment can never come up protected by a credential that is public
		// in the repository.
		AdminToken: "",
		DataDir:    "./data",
	}
}

// Validate reports configuration that would start an unsafe server.
func (c *Config) Validate() error {
	if c.AdminToken == "" {
		return errors.New("no admin token configured; set SUBREFINE_ADMIN_TOKEN (systemd: Environment=SUBREFINE_ADMIN_TOKEN=...)")
	}
	if c.TemplatesDir != "" {
		if st, err := os.Stat(c.TemplatesDir); err != nil || !st.IsDir() {
			return fmt.Errorf("SUBREFINE_TEMPLATES=%q is not a readable directory", c.TemplatesDir)
		}
	}
	return nil
}

// LoadFromEnv loads configuration overrides from environment variables.
func LoadFromEnv(cfg *Config) {
	if cfg == nil {
		return
	}
	if host := os.Getenv("SUBREFINE_HOST"); host != "" {
		cfg.Host = host
	}
	if portStr := os.Getenv("SUBREFINE_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p <= 65535 {
			cfg.Port = p
		}
	}
	if token := os.Getenv("SUBREFINE_ADMIN_TOKEN"); token != "" {
		cfg.AdminToken = token
	}
	if dataDir := os.Getenv("SUBREFINE_DATA_DIR"); dataDir != "" {
		cfg.DataDir = dataDir
	}
	if dir := os.Getenv("SUBREFINE_DASHBOARD"); dir != "" {
		cfg.DashboardDir = dir
	}
	if v := os.Getenv("SUBREFINE_TRUST_PROXY"); v != "" {
		if trust, err := strconv.ParseBool(v); err == nil {
			cfg.TrustProxy = trust
		}
	}
	if dir := os.Getenv("SUBREFINE_TEMPLATES"); dir != "" {
		cfg.TemplatesDir = dir
	}
}

func (c *Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}
