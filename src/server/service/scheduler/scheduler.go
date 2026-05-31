// Package scheduler provides a built-in task scheduler for recurring background jobs.
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/local/cassonic/src/server/metrics"
)

// JobFunc is a function that a scheduled job executes.
type JobFunc func(ctx context.Context) error

// Job describes a single recurring scheduled task.
type Job struct {
	// Name uniquely identifies the job for status reporting.
	Name string
	// Interval is how long to wait between runs.
	Interval time.Duration
	// RunAt is the absolute time for the first run; zero means run immediately on start.
	RunAt time.Time
	// Fn is the function to execute on each tick.
	Fn JobFunc
}

// jobEntry wraps a Job with runtime tracking state.
type jobEntry struct {
	Job
	lastRun   time.Time
	nextRun   time.Time
	running   bool
	lastError error
}

// JobStatus is a snapshot of a job's runtime state returned by Status.
type JobStatus struct {
	Name      string
	LastRun   time.Time
	NextRun   time.Time
	LastError string
	Running   bool
}

// Scheduler runs registered jobs on a 1-minute resolution tick loop.
type Scheduler struct {
	jobs   []*jobEntry
	logger *log.Logger
	mu     sync.Mutex
}

// New creates a Scheduler that logs to the provided logger.
func New(logger *log.Logger) *Scheduler {
	return &Scheduler{logger: logger}
}

// Register adds a job to the scheduler.
// Jobs must be registered before Start is called.
func (s *Scheduler) Register(j Job) {
	entry := &jobEntry{Job: j}
	if j.RunAt.IsZero() {
		entry.nextRun = time.Now()
	} else {
		entry.nextRun = j.RunAt
	}
	s.mu.Lock()
	s.jobs = append(s.jobs, entry)
	s.mu.Unlock()
}

// catchUpWindow is the maximum gap for which a missed job will be caught up on start.
const catchUpWindow = 24 * time.Hour

// Start launches the scheduler loop in a goroutine.
// It ticks every minute and fires any job whose nextRun has passed.
// The loop exits when ctx is cancelled.
// On startup, jobs whose nextRun is in the past within catchUpWindow are run immediately.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.catchUp(ctx)

		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		// Run the first check immediately so zero-RunAt jobs fire on startup.
		s.tick(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

// catchUp runs any job that missed its scheduled time within catchUpWindow.
func (s *Scheduler) catchUp(ctx context.Context) {
	now := time.Now()

	s.mu.Lock()
	var toRun []*jobEntry
	for _, entry := range s.jobs {
		if entry.nextRun.IsZero() {
			continue
		}
		if !now.After(entry.nextRun) {
			continue
		}
		gap := now.Sub(entry.nextRun)
		if gap >= catchUpWindow {
			continue
		}
		s.logger.Printf("[scheduler] catch-up: running %q (missed by %s)", entry.Name, gap.Round(time.Second))
		entry.running = true
		toRun = append(toRun, entry)
	}
	s.mu.Unlock()

	for _, entry := range toRun {
		captured := entry
		go func() {
			err := captured.Fn(ctx)

			outcome := "success"
			if err != nil {
				outcome = "error"
			}
			metrics.SchedulerRuns.WithLabelValues(captured.Name, outcome).Inc()

			s.mu.Lock()
			captured.lastRun = time.Now()
			captured.nextRun = captured.lastRun.Add(captured.Interval)
			captured.running = false
			captured.lastError = err
			s.mu.Unlock()

			if err != nil {
				s.logger.Printf("[scheduler] catch-up: job %q failed: %v", captured.Name, err)
			} else {
				s.logger.Printf("[scheduler] catch-up: job %q completed", captured.Name)
			}
		}()
	}
}

// tick inspects all registered jobs and spawns goroutines for any that are due.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range s.jobs {
		if entry.running {
			continue
		}
		if now.Before(entry.nextRun) {
			continue
		}

		entry.running = true
		captured := entry

		go func() {
			s.logger.Printf("[scheduler] starting job %q", captured.Name)
			err := captured.Fn(ctx)

			outcome := "success"
			if err != nil {
				outcome = "error"
			}
			metrics.SchedulerRuns.WithLabelValues(captured.Name, outcome).Inc()

			s.mu.Lock()
			captured.lastRun = time.Now()
			captured.nextRun = captured.lastRun.Add(captured.Interval)
			captured.running = false
			captured.lastError = err
			s.mu.Unlock()

			if err != nil {
				s.logger.Printf("[scheduler] job %q failed: %v", captured.Name, err)
			} else {
				s.logger.Printf("[scheduler] job %q completed", captured.Name)
			}
		}()
	}
}

// Status returns a snapshot of all registered jobs' current state.
func (s *Scheduler) Status() []JobStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]JobStatus, 0, len(s.jobs))
	for _, entry := range s.jobs {
		js := JobStatus{
			Name:    entry.Name,
			LastRun: entry.lastRun,
			NextRun: entry.nextRun,
			Running: entry.running,
		}
		if entry.lastError != nil {
			js.LastError = entry.lastError.Error()
		}
		out = append(out, js)
	}
	return out
}
