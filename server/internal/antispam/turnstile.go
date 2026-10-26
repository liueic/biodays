package antispam

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTurnstileURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	ErrTurnstileRejected    = errors.New("turnstile rejected")
	ErrTurnstileUnavailable = errors.New("turnstile unavailable")
)

type TurnstileVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

type TurnstileConfig struct {
	SecretKey string
	VerifyURL string
	Hostnames []string
	Timeout   time.Duration
}

type TurnstileClient struct {
	secret    string
	endpoint  string
	hostnames map[string]struct{}
	client    *http.Client
}

func NewTurnstileClient(cfg TurnstileConfig) *TurnstileClient {
	endpoint := strings.TrimSpace(cfg.VerifyURL)
	if endpoint == "" {
		endpoint = defaultTurnstileURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	hosts := make(map[string]struct{}, len(cfg.Hostnames))
	for _, host := range cfg.Hostnames {
		if trimmed := strings.ToLower(strings.TrimSpace(host)); trimmed != "" {
			hosts[trimmed] = struct{}{}
		}
	}
	return &TurnstileClient{
		secret:    cfg.SecretKey,
		endpoint:  endpoint,
		hostnames: hosts,
		client:    &http.Client{Timeout: timeout},
	}
}

type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
	Hostname   string   `json:"hostname"`
}

func (c *TurnstileClient) Verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return ErrTurnstileRejected
	}
	form := url.Values{
		"secret":   {c.secret},
		"response": {token},
	}
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrTurnstileUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return ErrTurnstileUnavailable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return ErrTurnstileUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return ErrTurnstileUnavailable
	}
	var parsed turnstileResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ErrTurnstileUnavailable
	}
	if !parsed.Success {
		return ErrTurnstileRejected
	}
	if len(c.hostnames) > 0 {
		if _, ok := c.hostnames[strings.ToLower(parsed.Hostname)]; !ok {
			return ErrTurnstileRejected
		}
	}
	return nil
}
