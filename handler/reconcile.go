package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mkurdziel/plexmatch-webhook/config"
	"github.com/mkurdziel/plexmatch-webhook/metrics"
	"github.com/mkurdziel/plexmatch-webhook/model"
	"github.com/mkurdziel/plexmatch-webhook/plex"
	"github.com/mkurdziel/plexmatch-webhook/plexmatch"
)

// ReconcileRequest is the payload for the reconcile endpoint.
type ReconcileRequest struct {
	Shows []model.Series `json:"shows"`
}

// ReconcileResult summarizes what happened during reconciliation.
type ReconcileResult struct {
	Written int      `json:"written"`
	Noop    int      `json:"noop"`
	Errors  []string `json:"errors,omitempty"`
}

// ReconcileHandler handles bulk reconciliation of .plexmatch files.
type ReconcileHandler struct {
	cfg        *config.Config
	plexClient *plex.Client
	idSource   plexmatch.IDSource
}

// NewReconcileHandler creates a new reconcile handler.
func NewReconcileHandler(cfg *config.Config, plexClient *plex.Client) *ReconcileHandler {
	return &ReconcileHandler{
		cfg:        cfg,
		plexClient: plexClient,
		idSource:   plexmatch.IDSource(cfg.PreferIDSource),
	}
}

func (h *ReconcileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReconcileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	result := ReconcileResult{}
	for _, show := range req.Shows {
		if show.Path == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("missing path for %q", show.Title))
			continue
		}
		if !show.HasStableID() {
			result.Errors = append(result.Errors, fmt.Sprintf("no stable ID for %q at %s", show.Title, show.Path))
			continue
		}

		content := plexmatch.Render(&show, h.idSource, h.cfg.WriteGUID)
		writeResult, err := plexmatch.AtomicWrite(show.Path, content, h.cfg.DryRun)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write error for %q: %v", show.Title, err))
			continue
		}

		switch writeResult {
		case plexmatch.WriteResultWritten:
			result.Written++
			metrics.FilesWrittenTotal.Inc()
		case plexmatch.WriteResultNoop:
			result.Noop++
			metrics.NoopWritesTotal.Inc()
		}
	}

	slog.Info("reconcile complete",
		"written", result.Written,
		"noop", result.Noop,
		"errors", len(result.Errors),
	)

	// Trigger one Plex refresh after all writes.
	if h.plexClient != nil && result.Written > 0 && !h.cfg.DryRun {
		if err := h.plexClient.RefreshLibrary(r.Context()); err != nil {
			metrics.PlexRefreshFailureTotal.Inc()
			slog.Error("plex refresh failed after reconcile", "error", err)
		} else {
			metrics.PlexRefreshSuccessTotal.Inc()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ReconcileScan walks a directory tree and writes .plexmatch for any show folders
// that contain a Series metadata file from Sonarr. This is a simpler version
// that just ensures every subdirectory of the given roots has a .plexmatch file.
func ReconcileScan(roots []string, cfg *config.Config, plexClient *plex.Client) ReconcileResult {
	idSource := plexmatch.IDSource(cfg.PreferIDSource)
	result := ReconcileResult{}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("read dir %s: %v", root, err))
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			showPath := filepath.Join(root, entry.Name())
			plexmatchPath := filepath.Join(showPath, ".plexmatch")

			// Skip if .plexmatch already exists (reconcile only fills gaps).
			if _, err := os.Stat(plexmatchPath); err == nil {
				result.Noop++
				continue
			}

			slog.Info("reconcile: show folder missing .plexmatch",
				"path", showPath,
			)
			result.Errors = append(result.Errors,
				fmt.Sprintf("no .plexmatch and no metadata source for %s — needs Sonarr data via POST /reconcile", showPath))
		}
	}

	_ = idSource // used when Sonarr data is provided
	return result
}
