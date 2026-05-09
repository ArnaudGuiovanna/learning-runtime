// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"testing"
	"time"

	"tutor-mcp/models"
)

func TestGetAutonomyMetrics_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerGetAutonomyMetrics, "", "get_autonomy_metrics", map[string]any{})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestGetAutonomyMetrics_HappyPath(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerGetAutonomyMetrics, "L_owner", "get_autonomy_metrics", map[string]any{})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if _, ok := out["score"]; !ok {
		t.Fatalf("expected score in result, got %v", out)
	}
	if _, ok := out["trend"]; !ok {
		t.Fatalf("expected trend in result, got %v", out)
	}
}

// TestGetAutonomyMetrics_DomainIDFiltersInteractions_Issue95 is the
// reproducer for issue #95: get_autonomy_metrics declared a `domain_id`
// parameter but ignored it. A learner who has data on D1 and asks
// `domain_id=D2` would see D1's signals attributed to D2.
//
// We trigger ProactiveReviewRate in D1: 3 review interactions all flagged
// is_proactive_review=true. With the bug, the D2-scoped call still sees
// proactive_review_rate ≈ 1.0. With the fix it must be 0.0 (no D2
// interactions).
func TestGetAutonomyMetrics_DomainIDFiltersInteractions_Issue95(t *testing.T) {
	store, deps := setupToolsTest(t)

	d1, err := store.CreateDomain("L_owner", "d1", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	})
	if err != nil {
		t.Fatalf("create d1: %v", err)
	}
	d2, err := store.CreateDomain("L_owner", "d2", "", models.KnowledgeSpace{
		Concepts: []string{"x"},
	})
	if err != nil {
		t.Fatalf("create d2: %v", err)
	}

	// Seed 3 proactive review interactions on D1's concept "a", recent
	// enough to fall inside the 30-day window the handler uses.
	now := time.Now().UTC()
	for k := 0; k < 3; k++ {
		ts := now.Add(time.Duration(-k) * time.Hour)
		_, err := store.RawDB().Exec(
			`INSERT INTO interactions (learner_id, concept, activity_type, success, response_time, confidence, error_type, notes, hints_requested, self_initiated, calibration_id, is_proactive_review, misconception_type, misconception_detail, domain_id, bkt_slip, bkt_guess, created_at)
			 VALUES (?, 'a', 'RECALL_EXERCISE', 1, 60, 0.5, '', '', 0, 0, '', 1, NULL, NULL, ?, NULL, NULL, ?)`,
			"L_owner", d1.ID, ts,
		)
		if err != nil {
			t.Fatalf("seed interaction %d: %v", k, err)
		}
	}

	// Sanity: empty domain_id (learner-wide) sees the full proactive
	// signal — confirms the seed is sufficient.
	resAll := callTool(t, deps, registerGetAutonomyMetrics, "L_owner", "get_autonomy_metrics", map[string]any{})
	if resAll.IsError {
		t.Fatalf("learner-wide call errored: %q", resultText(resAll))
	}
	all := decodeResult(t, resAll)
	allRate, ok := all["proactive_review_rate"].(float64)
	if !ok {
		t.Fatalf("expected float proactive_review_rate, got %T (%v)", all["proactive_review_rate"], all)
	}
	if allRate <= 0 {
		t.Fatalf("learner-wide call should pick up the D1 proactive signal (precondition): got rate=%v", allRate)
	}

	// Scoped to D2: no D2 interactions → rate must collapse to 0.
	resD2 := callTool(t, deps, registerGetAutonomyMetrics, "L_owner", "get_autonomy_metrics", map[string]any{
		"domain_id": d2.ID,
	})
	if resD2.IsError {
		t.Fatalf("d2-scoped call errored: %q", resultText(resD2))
	}
	d2out := decodeResult(t, resD2)
	d2Rate, ok := d2out["proactive_review_rate"].(float64)
	if !ok {
		t.Fatalf("expected float proactive_review_rate in D2 result, got %T (%v)", d2out["proactive_review_rate"], d2out)
	}
	if d2Rate != 0 {
		t.Fatalf("issue #95: domain_id=%s must scope to D2's concept set, expected proactive_review_rate=0, got %v", d2.ID, d2Rate)
	}
}

// TestGetAutonomyMetrics_UnknownDomainIDRejected_Issue95 asserts the
// "domain not found" guard for explicit-but-foreign domain_ids.
func TestGetAutonomyMetrics_UnknownDomainIDRejected_Issue95(t *testing.T) {
	store, deps := setupToolsTest(t)
	foreign, err := store.CreateDomain("L_attacker", "secret", "", models.KnowledgeSpace{
		Concepts: []string{"z"},
	})
	if err != nil {
		t.Fatalf("create foreign domain: %v", err)
	}
	res := callTool(t, deps, registerGetAutonomyMetrics, "L_owner", "get_autonomy_metrics", map[string]any{
		"domain_id": foreign.ID,
	})
	if !res.IsError {
		t.Fatalf("expected IsError on foreign domain_id, got payload=%q", resultText(res))
	}
	if got := resultText(res); got != "domain not found" {
		t.Fatalf("expected error %q, got %q", "domain not found", got)
	}
}

// TestGetAutonomyMetrics_EmptyDomainIDStaysLearnerWide guards the
// contract that an empty domain_id keeps the existing learner-wide
// behaviour, so callers that don't pass anything don't regress.
func TestGetAutonomyMetrics_EmptyDomainIDStaysLearnerWide(t *testing.T) {
	store, deps := setupToolsTest(t)

	d1, err := store.CreateDomain("L_owner", "d1", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	})
	if err != nil {
		t.Fatalf("create d1: %v", err)
	}
	now := time.Now().UTC()
	for k := 0; k < 3; k++ {
		ts := now.Add(time.Duration(-k) * time.Hour)
		_, err := store.RawDB().Exec(
			`INSERT INTO interactions (learner_id, concept, activity_type, success, response_time, confidence, error_type, notes, hints_requested, self_initiated, calibration_id, is_proactive_review, misconception_type, misconception_detail, domain_id, bkt_slip, bkt_guess, created_at)
			 VALUES (?, 'a', 'RECALL_EXERCISE', 1, 60, 0.5, '', '', 0, 0, '', 1, NULL, NULL, ?, NULL, NULL, ?)`,
			"L_owner", d1.ID, ts,
		)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	res := callTool(t, deps, registerGetAutonomyMetrics, "L_owner", "get_autonomy_metrics", map[string]any{})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	rate, ok := out["proactive_review_rate"].(float64)
	if !ok {
		t.Fatalf("expected proactive_review_rate, got %v", out)
	}
	if rate <= 0 {
		t.Fatalf("empty domain_id must keep learner-wide behaviour, expected rate>0, got %v", rate)
	}
}
