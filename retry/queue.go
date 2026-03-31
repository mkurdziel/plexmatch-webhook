package retry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mkurdziel/plexmatch-webhook/metrics"
)

// Task represents a retriable unit of work.
type Task struct {
	Name    string
	Fn      func() error
	Attempt int
}

// Queue is a simple in-memory retry queue with exponential backoff.
type Queue struct {
	maxAttempts int
	baseDelay   time.Duration
	mu          sync.Mutex
	tasks       []Task
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewQueue creates a retry queue.
func NewQueue(maxAttempts int, baseDelay time.Duration) *Queue {
	return &Queue{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
	}
}

// Enqueue adds a task to the retry queue.
func (q *Queue) Enqueue(t Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, t)
	metrics.RetryEnqueuedTotal.Inc()
}

// Start begins processing the retry queue in the background.
func (q *Queue) Start(ctx context.Context) {
	ctx, q.cancel = context.WithCancel(ctx)
	q.wg.Add(1)
	go q.run(ctx)
}

// Stop signals the queue to stop and waits for it to finish.
func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
}

func (q *Queue) run(ctx context.Context) {
	defer q.wg.Done()

	ticker := time.NewTicker(q.baseDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.processPending()
		}
	}
}

func (q *Queue) processPending() {
	q.mu.Lock()
	if len(q.tasks) == 0 {
		q.mu.Unlock()
		return
	}
	// Take all current tasks.
	tasks := q.tasks
	q.tasks = nil
	q.mu.Unlock()

	var requeue []Task
	for _, t := range tasks {
		if err := t.Fn(); err != nil {
			t.Attempt++
			if t.Attempt < q.maxAttempts {
				slog.Warn("retry task failed, will retry",
					"task", t.Name,
					"attempt", t.Attempt,
					"max", q.maxAttempts,
					"error", err,
				)
				requeue = append(requeue, t)
			} else {
				slog.Error("retry task exhausted attempts",
					"task", t.Name,
					"attempts", t.Attempt,
					"error", err,
				)
			}
		} else {
			slog.Info("retry task succeeded", "task", t.Name, "attempt", t.Attempt)
		}
	}

	if len(requeue) > 0 {
		q.mu.Lock()
		q.tasks = append(q.tasks, requeue...)
		q.mu.Unlock()
	}
}
