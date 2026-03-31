package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkurdziel/plexmatch-webhook/config"
	"github.com/mkurdziel/plexmatch-webhook/retry"
)

func newTestConfig(secret string) *config.Config {
	return &config.Config{
		WebhookSecret:  secret,
		PreferIDSource: "tvdb",
		WriteGUID:      true,
		DryRun:         false,
		AllowedEventTypes: map[string]bool{
			"Download": true,
			"Upgrade":  true,
			"Rename":   true,
		},
	}
}

func newTestHandler(cfg *config.Config) *WebhookHandler {
	q := retry.NewQueue(3, 1*time.Second)
	return NewWebhookHandler(cfg, nil, q)
}

func postJSON(handler http.Handler, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/sonarr/webhook", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestWebhook_Unauthorized(t *testing.T) {
	h := newTestHandler(newTestConfig("mysecret"))
	rec := postJSON(h, map[string]interface{}{}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWebhook_CorrectSecret(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig("mysecret")
	h := newTestHandler(cfg)

	payload := map[string]interface{}{
		"eventType": "Download",
		"series": map[string]interface{}{
			"path":   dir,
			"title":  "Test Show",
			"year":   2024,
			"tvdbId": 12345,
		},
	}
	rec := postJSON(h, payload, map[string]string{"X-Webhook-Secret": "mysecret"})
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Check file was written.
	got, err := os.ReadFile(filepath.Join(dir, ".plexmatch"))
	if err != nil {
		t.Fatalf("read .plexmatch: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty .plexmatch")
	}
}

func TestWebhook_IgnoredEvent(t *testing.T) {
	h := newTestHandler(newTestConfig(""))
	payload := map[string]interface{}{
		"eventType": "Grab",
		"series": map[string]interface{}{
			"path":   "/tmp/test",
			"tvdbId": 123,
		},
	}
	rec := postJSON(h, payload, nil)
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
}

func TestWebhook_MissingSeries(t *testing.T) {
	h := newTestHandler(newTestConfig(""))
	payload := map[string]interface{}{
		"eventType": "Download",
	}
	rec := postJSON(h, payload, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestWebhook_MissingPath(t *testing.T) {
	h := newTestHandler(newTestConfig(""))
	payload := map[string]interface{}{
		"eventType": "Download",
		"series": map[string]interface{}{
			"tvdbId": 123,
		},
	}
	rec := postJSON(h, payload, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestWebhook_NoStableID(t *testing.T) {
	h := newTestHandler(newTestConfig(""))
	payload := map[string]interface{}{
		"eventType": "Download",
		"series": map[string]interface{}{
			"path":  "/tmp/test",
			"title": "No ID Show",
		},
	}
	rec := postJSON(h, payload, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestWebhook_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(newTestConfig(""))
	req := httptest.NewRequest(http.MethodGet, "/sonarr/webhook", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestWebhook_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfg := newTestConfig("")
	cfg.DryRun = true
	h := newTestHandler(cfg)

	payload := map[string]interface{}{
		"eventType": "Download",
		"series": map[string]interface{}{
			"path":   dir,
			"title":  "Dry Run Show",
			"year":   2024,
			"tvdbId": 99999,
		},
	}
	rec := postJSON(h, payload, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// File should not exist in dry-run.
	if _, err := os.Stat(filepath.Join(dir, ".plexmatch")); !os.IsNotExist(err) {
		t.Error("file should not exist in dry-run mode")
	}
}
