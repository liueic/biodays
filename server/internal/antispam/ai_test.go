package antispam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClassifier(t *testing.T, handler http.HandlerFunc, style string, timeout time.Duration) *CloudflareClassifier {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	classifier, err := NewCloudflareClassifier(CloudflareConfig{
		AccountID: "account", APIToken: "token", Model: "@cf/cloudflare/clef-flash",
		Style: style, Timeout: timeout, BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("create classifier: %v", err)
	}
	return classifier
}

func TestClassifyDecision(t *testing.T) {
	classifier := newTestClassifier(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"answers":{"label":{"probabilities":{"spam":0.91,"legit":0.07,"uncertain":0.02}},"is_spam":0.91}}}`))
	}, "decision", time.Second)

	verdict, err := classifier.Classify(context.Background(), "加微信 看更多")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if verdict.Label != "spam" || verdict.Confidence < 0.9 {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
}

func TestClassifyChatJSON(t *testing.T) {
	classifier := newTestClassifier(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := "```json\n{\"label\":\"legit\",\"confidence\":0.88,\"reasons\":[\"topic suggestion\"]}\n```"
		payload, _ := json.Marshal(map[string]any{
			"success": true,
			"result":  map[string]string{"response": response},
		})
		_, _ = w.Write(payload)
	}, "chat", time.Second)

	verdict, err := classifier.Classify(context.Background(), "想了解 PCR 的历史")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if verdict.Label != "legit" || verdict.Confidence < 0.8 {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
}

func TestClassifyBadJSONIsUncertain(t *testing.T) {
	classifier := newTestClassifier(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"response":"not json"}}`))
	}, "chat", time.Second)

	verdict, err := classifier.Classify(context.Background(), "test")
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if verdict.Label != "uncertain" {
		t.Fatalf("expected uncertain, got %+v", verdict)
	}
}

func TestClassifyServerError(t *testing.T) {
	classifier := newTestClassifier(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}, "decision", time.Second)

	_, err := classifier.Classify(context.Background(), "test")
	if !errors.Is(err, ErrClassifierUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestClassifyTimeout(t *testing.T) {
	classifier := newTestClassifier(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"success":true,"result":{}}`))
	}, "decision", 50*time.Millisecond)

	_, err := classifier.Classify(context.Background(), "test")
	if !errors.Is(err, ErrClassifierUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
