package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mkurdziel/plexmatch-webhook/config"
	"github.com/mkurdziel/plexmatch-webhook/handler"
	"github.com/mkurdziel/plexmatch-webhook/plex"
	"github.com/mkurdziel/plexmatch-webhook/retry"
)

func main() {
	// Structured JSON logging.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("configuration loaded",
		"addr", cfg.Addr(),
		"dryRun", cfg.DryRun,
		"preferIDSource", cfg.PreferIDSource,
		"writeGUID", cfg.WriteGUID,
		"plexEnabled", cfg.PlexEnabled(),
	)

	// Plex client (nil if not configured).
	var plexClient *plex.Client
	if cfg.PlexEnabled() {
		plexClient = plex.NewClient(cfg.PlexBaseURL, cfg.PlexToken, cfg.PlexSectionID)
		slog.Info("plex integration enabled", "baseURL", cfg.PlexBaseURL, "sectionID", cfg.PlexSectionID)
	}

	// Retry queue.
	retryQueue := retry.NewQueue(5, 10*time.Second)
	retryQueue.Start(context.Background())

	// Handlers.
	mux := http.NewServeMux()
	mux.Handle("/sonarr/webhook", handler.NewWebhookHandler(cfg, plexClient, retryQueue))
	mux.Handle("/reconcile", handler.NewReconcileHandler(cfg, plexClient))
	mux.Handle("/healthz", &handler.HealthHandler{})
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	retryQueue.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
