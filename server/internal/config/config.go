package config

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port           int      `env:"BIODAYS_PORT" envDefault:"8080"`
	DBPath         string   `env:"BIODAYS_DB_PATH" envDefault:"./biodays.db"`
	DataDir        string   `env:"BIODAYS_DATA_DIR"`
	CORSOrigins    []string `env:"BIODAYS_CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	TrustedProxies []string `env:"BIODAYS_TRUSTED_PROXIES" envSeparator:","`
	RateLimit      int      `env:"BIODAYS_RATE_LIMIT" envDefault:"60"`
	LogLevel       string   `env:"BIODAYS_LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("BIODAYS_PORT must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.RateLimit < 1 {
		return Config{}, fmt.Errorf("BIODAYS_RATE_LIMIT must be positive, got %d", cfg.RateLimit)
	}
	origins := make([]string, 0, len(cfg.CORSOrigins))
	for _, origin := range cfg.CORSOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	cfg.CORSOrigins = origins

	trustedProxies, err := parseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return Config{}, err
	}
	cfg.TrustedProxies = trustedProxies

	if _, err := ParseLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseTrustedProxies(values []string) ([]string, error) {
	proxies := make([]string, 0, len(values))
	for _, value := range values {
		proxy := strings.TrimSpace(value)
		if proxy == "" {
			continue
		}
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return nil, fmt.Errorf("BIODAYS_TRUSTED_PROXIES contains invalid IP or CIDR %q", proxy)
			}
		}
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

func ParseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(value))); err != nil {
		return slog.LevelInfo, fmt.Errorf("invalid BIODAYS_LOG_LEVEL %q: %w", value, err)
	}
	return level, nil
}
