package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mkurdziel/plexmatch-webhook/config"
	"github.com/mkurdziel/plexmatch-webhook/metrics"
	"github.com/mkurdziel/plexmatch-webhook/model"
	"github.com/mkurdziel/plexmatch-webhook/plex"
	"github.com/mkurdziel/plexmatch-webhook/plexmatch"
	"github.com/mkurdziel/plexmatch-webhook/retry"
)

// WebhookHandler handles incoming Sonarr webhook requests.
type WebhookHandler struct {
	cfg        *config.Config
	plexClient *plex.Client
	retryQueue *retry.Queue
	idSource   plexmatch.IDSource
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(cfg *config.Config, plexClient *plex.Client, retryQueue *retry.Queue) *WebhookHandler {
	return &WebhookHandler{
		cfg:        cfg,
		plexClient: plexClient,
		retryQueue: retryQueue,
		idSource:   plexmatch.IDSource(cfg.PreferIDSource),
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics.WebhookRequestsTotal.Inc()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate auth.
	if h.cfg.WebhookSecret != "" {
		secret := r.Header.Get("X-Webhook-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.cfg.WebhookSecret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Validate content type.
	ct := r.Header.Get("Content-Type")
	if ct != "" && ct != "application/json" {
		metrics.MalformedPayloadsTotal.Inc()
		http.Error(w, "content-type must be application/json", http.StatusBadRequest)
		return
	}

	// Parse payload.
	var payload model.SonarrWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		metrics.MalformedPayloadsTotal.Inc()
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	log := slog.With("eventType", payload.EventType)

	// Check event type.
	if !h.cfg.AllowedEventTypes[payload.EventType] {
		metrics.IgnoredEventsTotal.Inc()
		log.Info("ignoring irrelevant event type")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, "event type ignored")
		return
	}

	// Validate required fields.
	if payload.Series == nil {
		metrics.MalformedPayloadsTotal.Inc()
		http.Error(w, "missing series object", http.StatusBadRequest)
		return
	}

	if payload.Series.Path == "" {
		metrics.MalformedPayloadsTotal.Inc()
		http.Error(w, "missing series.path", http.StatusBadRequest)
		return
	}

	if !payload.Series.HasStableID() {
		metrics.MalformedPayloadsTotal.Inc()
		log.Warn("no stable ID found, skipping",
			"title", payload.Series.Title,
			"path", payload.Series.Path,
		)
		http.Error(w, "no stable ID (tvdbId, tmdbId, or imdbId) provided", http.StatusBadRequest)
		return
	}

	provider, value := plexmatch.SelectBestID(payload.Series, h.idSource)
	log = log.With(
		"title", payload.Series.Title,
		"year", payload.Series.Year,
		"path", payload.Series.Path,
		"idSource", provider,
		"idValue", value,
	)

	// Render .plexmatch content.
	content := plexmatch.Render(payload.Series, h.idSource, h.cfg.WriteGUID)

	// Write the file.
	result, err := plexmatch.AtomicWrite(payload.Series.Path, content, h.cfg.DryRun)
	switch result {
	case plexmatch.WriteResultWritten:
		metrics.FilesWrittenTotal.Inc()
		log.Info(".plexmatch written")
	case plexmatch.WriteResultNoop:
		metrics.NoopWritesTotal.Inc()
		log.Info(".plexmatch unchanged")
	case plexmatch.WriteResultDryRun:
		log.Info(".plexmatch would be written (dry-run)")
	case plexmatch.WriteResultNoFolder:
		log.Warn("series folder missing, enqueuing retry", "error", err)
		h.enqueueWrite(payload.Series, content)
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, "folder missing, enqueued for retry")
		return
	}

	if err != nil && result != plexmatch.WriteResultNoFolder {
		log.Error("failed to write .plexmatch", "error", err)
		http.Error(w, "failed to write .plexmatch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger Plex refresh.
	if h.plexClient != nil && result == plexmatch.WriteResultWritten && !h.cfg.DryRun {
		if err := h.plexClient.RefreshLibrary(r.Context()); err != nil {
			metrics.PlexRefreshFailureTotal.Inc()
			log.Error("plex refresh failed, enqueuing retry", "error", err)
			h.enqueueRefresh()
		} else {
			metrics.PlexRefreshSuccessTotal.Inc()
			log.Info("plex library refresh triggered")
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (h *WebhookHandler) enqueueWrite(series *model.Series, content string) {
	path := series.Path
	h.retryQueue.Enqueue(retry.Task{
		Name: fmt.Sprintf("write-%s", path),
		Fn: func() error {
			result, err := plexmatch.AtomicWrite(path, content, h.cfg.DryRun)
			if err != nil {
				return err
			}
			if result == plexmatch.WriteResultWritten {
				metrics.FilesWrittenTotal.Inc()
				slog.Info("retry: .plexmatch written", "path", path)
				if h.plexClient != nil && !h.cfg.DryRun {
					if refreshErr := h.plexClient.RefreshLibrary(context.Background()); refreshErr != nil {
						metrics.PlexRefreshFailureTotal.Inc()
						slog.Error("retry: plex refresh failed", "error", refreshErr)
					} else {
						metrics.PlexRefreshSuccessTotal.Inc()
					}
				}
			}
			return nil
		},
	})
}

func (h *WebhookHandler) enqueueRefresh() {
	h.retryQueue.Enqueue(retry.Task{
		Name: "plex-refresh",
		Fn: func() error {
			if err := h.plexClient.RefreshLibrary(context.Background()); err != nil {
				return err
			}
			metrics.PlexRefreshSuccessTotal.Inc()
			return nil
		},
	})
}
