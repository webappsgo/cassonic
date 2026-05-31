package scheduler

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func TestRegisterAndStatus(t *testing.T) {
	s := New(silentLogger())

	s.Register(Job{
		Name:     "test-job",
		Interval: time.Hour,
		RunAt:    time.Now().Add(time.Hour),
		Fn: func(ctx context.Context) error {
			return nil
		},
	})

	statuses := s.Status()
	if len(statuses) != 1 {
		t.Fatalf("Status: got %d jobs, want 1", len(statuses))
	}
	if statuses[0].Name != "test-job" {
		t.Errorf("Status[0].Name: got %q, want %q", statuses[0].Name, "test-job")
	}
}

func TestMultipleRegistrations(t *testing.T) {
	s := New(silentLogger())

	for i := 0; i < 5; i++ {
		name := "job-" + string(rune('a'+i))
		s.Register(Job{
			Name:     name,
			Interval: time.Hour,
			RunAt:    time.Now().Add(time.Hour),
			Fn: func(ctx context.Context) error {
				return nil
			},
		})
	}

	statuses := s.Status()
	if len(statuses) != 5 {
		t.Errorf("Status: got %d jobs, want 5", len(statuses))
	}
}

func TestJobRunsOnStart(t *testing.T) {
	s := New(silentLogger())

	var counter int64

	s.Register(Job{
		Name:     "counter-job",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&counter) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("job never ran within timeout; counter = %d", atomic.LoadInt64(&counter))
}

func TestJobNotRunBeforeRunAt(t *testing.T) {
	s := New(silentLogger())

	var counter int64

	s.Register(Job{
		Name:     "future-job",
		Interval: time.Hour,
		RunAt:    time.Now().Add(10 * time.Minute),
		Fn: func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	if c := atomic.LoadInt64(&counter); c != 0 {
		t.Errorf("future job ran %d times but should not have run yet", c)
	}
}

func TestJobRunsAtMostOnceConcurrently(t *testing.T) {
	s := New(silentLogger())

	var mu sync.Mutex
	concurrentRuns := 0
	maxConcurrent := 0
	var wg sync.WaitGroup

	s.Register(Job{
		Name:     "concurrent-job",
		Interval: time.Millisecond,
		Fn: func(ctx context.Context) error {
			wg.Add(1)
			defer wg.Done()

			mu.Lock()
			concurrentRuns++
			if concurrentRuns > maxConcurrent {
				maxConcurrent = concurrentRuns
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			concurrentRuns--
			mu.Unlock()

			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	s.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()

	mu.Lock()
	max := maxConcurrent
	mu.Unlock()

	if max > 1 {
		t.Errorf("job ran %d concurrent instances, want at most 1", max)
	}
}

func TestJobErrorRecordedInStatus(t *testing.T) {
	s := New(silentLogger())

	expectedErr := errors.New("job failed")

	s.Register(Job{
		Name:     "failing-job",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			return expectedErr
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statuses := s.Status()
		if len(statuses) > 0 && statuses[0].LastError != "" {
			if statuses[0].LastError != expectedErr.Error() {
				t.Errorf("LastError: got %q, want %q", statuses[0].LastError, expectedErr.Error())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Error("failing job LastError never populated within timeout")
}

func TestStatusRunningField(t *testing.T) {
	s := New(silentLogger())

	started := make(chan struct{})
	proceed := make(chan struct{})

	s.Register(Job{
		Name:     "slow-job",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			close(started)
			<-proceed
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("job never started")
	}

	statuses := s.Status()
	if len(statuses) == 0 {
		t.Fatal("no statuses returned")
	}
	if !statuses[0].Running {
		t.Error("Running should be true while job is executing")
	}

	close(proceed)
}

func TestContextCancellationStopsScheduler(t *testing.T) {
	s := New(silentLogger())

	var runCount int64

	s.Register(Job{
		Name:     "ctx-job",
		Interval: time.Hour,
		RunAt:    time.Now().Add(time.Hour),
		Fn: func(ctx context.Context) error {
			atomic.AddInt64(&runCount, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	cancel()

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt64(&runCount) != 0 {
		t.Errorf("future job ran %d times after context cancel", runCount)
	}
}
