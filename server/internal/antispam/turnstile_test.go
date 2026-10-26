package antispam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTurnstileVerify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("secret") != "secret" || r.Form.Get("response") != "token" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		_, _ = w.Write([]byte(`{"success":true,"hostname":"biodays.test"}`))
	}))
	t.Cleanup(server.Close)

	client := NewTurnstileClient(TurnstileConfig{
		SecretKey: "secret", VerifyURL: server.URL, Hostnames: []string{"biodays.test"}, Timeout: time.Second,
	})
	if err := client.Verify(context.Background(), "token", "203.0.113.10"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestTurnstileRejectsWrongHostname(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"hostname":"evil.test"}`))
	}))
	t.Cleanup(server.Close)

	client := NewTurnstileClient(TurnstileConfig{
		SecretKey: "secret", VerifyURL: server.URL, Hostnames: []string{"biodays.test"}, Timeout: time.Second,
	})
	if err := client.Verify(context.Background(), "token", ""); !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected rejected, got %v", err)
	}
}

func TestTurnstileRejectsFailedChallenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	t.Cleanup(server.Close)

	client := NewTurnstileClient(TurnstileConfig{
		SecretKey: "secret", VerifyURL: server.URL, Timeout: time.Second,
	})
	if err := client.Verify(context.Background(), "token", ""); !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected rejected, got %v", err)
	}
}
