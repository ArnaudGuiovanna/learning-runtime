// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package memory

import (
	"strings"
	"testing"
	"time"
)

func TestPrepareJobsMonthlyWindow(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ROOT", t.TempDir())
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "true")

	jobs, err := PrepareJobs("L1", time.Date(2026, time.May, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("PrepareJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %#v, want one monthly job", jobs)
	}
	if jobs[0].Period != PeriodMonthly || jobs[0].PeriodKey != "2026-04" {
		t.Fatalf("unexpected job: %#v", jobs[0])
	}
}

func TestConsolidationPayloadHelpersDoNotCreateArchives(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ROOT", t.TempDir())
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "true")
	t.Setenv("TUTOR_MCP_CONSOLIDATION_SEED", "42")

	for _, ts := range []time.Time{
		time.Date(2026, time.April, 10, 9, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 11, 9, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC),
	} {
		content := `---
timestamp: ` + ts.Format(time.RFC3339) + `
concepts_touched: ["bayes"]
novelty_flag: false
---

## Summary
Session on bayes at ` + ts.Format("2006-01-02")
		if err := Write(WriteRequest{LearnerID: "L1", Scope: ScopeSession, Timestamp: ts, Operation: OpReplaceFile, Content: content}); err != nil {
			t.Fatalf("Write session: %v", err)
		}
	}

	job := ConsolidationJob{
		LearnerID: "L1",
		Period:    PeriodMonthly,
		PeriodKey: "2026-04",
		StartDate: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	}
	sessions, err := SessionsInRange(job.LearnerID, job.StartDate, job.EndDate)
	if err != nil {
		t.Fatalf("SessionsInRange: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}
	replay, err := InterleavedReplaySessions(job.LearnerID, job.StartDate, 3)
	if err != nil {
		t.Fatalf("InterleavedReplaySessions: %v", err)
	}
	if len(replay) != 1 {
		t.Fatalf("replay = %d, want 1", len(replay))
	}
	concepts := ConceptsFromSessions(sessions)
	if len(concepts) != 1 || concepts[0] != "bayes" {
		t.Fatalf("concepts = %#v, want bayes", concepts)
	}
	body, _ := Read("L1", ScopeArchive, "2026-04")
	if strings.TrimSpace(body) != "" {
		t.Fatalf("payload helpers must not create archive content, got %q", body)
	}
}
