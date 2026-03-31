package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WebhookRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_webhook_requests_total",
		Help: "Total number of webhook requests received.",
	})

	IgnoredEventsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_ignored_events_total",
		Help: "Total number of ignored events (irrelevant event types).",
	})

	MalformedPayloadsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_malformed_payloads_total",
		Help: "Total number of malformed payloads.",
	})

	FilesWrittenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_files_written_total",
		Help: "Total number of .plexmatch files written.",
	})

	NoopWritesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_noop_writes_total",
		Help: "Total number of no-op writes (content unchanged).",
	})

	PlexRefreshSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_plex_refresh_success_total",
		Help: "Total number of successful Plex library refreshes.",
	})

	PlexRefreshFailureTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_plex_refresh_failure_total",
		Help: "Total number of failed Plex library refreshes.",
	})

	RetryEnqueuedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "plexmatch_retry_enqueued_total",
		Help: "Total number of tasks enqueued for retry.",
	})
)
