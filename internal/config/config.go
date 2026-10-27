package config

import (
	"net"
	"os"
	"strconv"
)

type Config struct {
	Host       string `json:"host" yaml:"host"`
	Port       int    `json:"port" yaml:"port"`
	AdminToken string `json:"admin_token" yaml:"admin_token"`
	DataDir    string `json:"data_dir" yaml:"data_dir"`
}

func DefaultConfig() *Config {
	return &Config{
		Host:       "0.0.0.0",
		Port:       3000,
		AdminToken: "subrefine_secret_token",
		DataDir:    "./data",
	}
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
}

func (c *Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}
