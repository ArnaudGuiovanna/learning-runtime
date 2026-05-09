// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSchedulerStop_WaitsForInFlightJobs_Issue123 reproduces the issue #123
// bug: Stop() previously discarded the context returned by cron.Stop(), which
// is the only signal that in-flight jobs have drained. As a result, main.go's
// deferred shutdown chain could run database.Close() while a cron tick was
// still mid-iteration over learners, producing "sql: database is closed"
// mid-loop and silent data loss on webhook_message_queue + scheduled_alerts.
//
// Fix: Stop() must block until the cron context is Done (or a bounded timeout
// elapses).
//
// Test strategy: register a job that takes ~150ms, start the scheduler so it
// fires once, then call Stop() while the job is in-flight. Stop() must block
// for at least the remaining job duration. We allow generous slack to keep
// this stable on CI under load.
func TestSchedulerStop_WaitsForInFlightJobs_Issue123(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewScheduler(nil, logger)

	const jobDuration = 150 * time.Millisecond

	var (
		started  = make(chan struct{}, 1)
		finished int32
		once     sync.Once
	)

	if _, err := s.cron.AddFunc("@every 50ms", func() {
		// Only the first tick should block; subsequent ticks (if any) are no-ops.
		fired := false
		once.Do(func() {
			fired = true
		})
		if !fired {
			return
		}
		started <- struct{}{}
		time.Sleep(jobDuration)
		atomic.StoreInt32(&finished, 1)
	}); err != nil {
		t.Fatalf("add job: %v", err)
	}

	s.cron.Start()

	// Wait for the job to actually be in-flight before we call Stop().
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("job never started; cron not ticking?")
	}

	// Now Stop() must block until the in-flight job finishes.
	begin := time.Now()
	s.Stop()
	elapsed := time.Since(begin)

	if atomic.LoadInt32(&finished) == 0 {
		t.Fatalf("Stop() returned before the in-flight job finished — deferred shutdown would race with running jobs (issue #123)")
	}

	// Allow 50ms slack on the lower bound for scheduler/wakeup jitter; we mainly
	// want to assert Stop() is not instantaneous when a job is in-flight.
	const minBlock = 100 * time.Millisecond
	if elapsed < minBlock {
		t.Fatalf("Stop() returned in %v, expected at least %v while a %v job was in-flight", elapsed, minBlock, jobDuration)
	}
}

// TestSchedulerStop_TimesOutOnRunawayJob_Issue123 verifies the safety valve:
// if a job is wedged forever, Stop() must not hang the shutdown indefinitely.
// The production timeout is 25s — too slow for CI — so we shrink stopTimeout
// for the test (the field is package-private, set directly here).
func TestSchedulerStop_TimesOutOnRunawayJob_Issue123(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewScheduler(nil, logger)
	s.stopTimeout = 50 * time.Millisecond

	wedge := make(chan struct{})
	defer close(wedge) // unblock the wedged job at end of test

	started := make(chan struct{}, 1)
	var once sync.Once
	if _, err := s.cron.AddFunc("@every 25ms", func() {
		fired := false
		once.Do(func() { fired = true })
		if !fired {
			return
		}
		started <- struct{}{}
		<-wedge // never returns until the test cleans up
	}); err != nil {
		t.Fatalf("add wedged job: %v", err)
	}

	s.cron.Start()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("wedged job never started")
	}

	begin := time.Now()
	s.Stop()
	elapsed := time.Since(begin)

	// We expect Stop() to return after roughly stopTimeout — not block forever.
	// Upper bound is generous to absorb CI scheduling jitter.
	if elapsed > 2*time.Second {
		t.Fatalf("Stop() took %v with stopTimeout=50ms — timeout safety valve appears broken", elapsed)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("Stop() returned in %v — should have waited at least the configured stopTimeout (50ms)", elapsed)
	}
}
