package config_test

import (
	"testing"

	"biodays/server/internal/config"

	"github.com/stretchr/testify/require"
)

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", " 127.0.0.1 , 10.0.0.0/8, ,::1 ")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.1", "10.0.0.0/8", "::1"}, cfg.TrustedProxies)
}

func TestLoadEmptyTrustedProxies(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.TrustedProxies)
}

func TestLoadRejectsInvalidTrustedProxy(t *testing.T) {
	t.Setenv("BIODAYS_TRUSTED_PROXIES", "not-an-ip")

	_, err := config.Load()
	require.ErrorContains(t, err, "BIODAYS_TRUSTED_PROXIES")
}
