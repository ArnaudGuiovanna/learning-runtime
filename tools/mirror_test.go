// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"testing"
	"time"

	"tutor-mcp/models"
)

func TestGetMetacognitiveMirror_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerGetMetacognitiveMirror, "", "get_metacognitive_mirror", map[string]any{})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestGetMetacognitiveMirror_NoPattern(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerGetMetacognitiveMirror, "L_owner", "get_metacognitive_mirror", map[string]any{})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if _, ok := out["mirror"]; !ok {
		t.Fatalf("expected mirror key, got %v", out)
	}
}

// TestGetMetacognitiveMirror_DomainIDFiltersInteractions_Issue95 is the
// reproducer for issue #95: the handler declared a `domain_id` parameter
// but ignored it, surfacing a learner-wide pattern that the LLM and the
// caller would mistakenly attribute to the requested domain.
//
// Setup: learner has two domains. D1 has enough interactions to trigger
// the "no_initiative" pattern (>=3 sessions, none self-initiated); D2 is
// empty. A `get_metacognitive_mirror(domain_id=D2)` call must return a
// nil mirror — pre-fix, it would leak D1's pattern.
func TestGetMetacognitiveMirror_DomainIDFiltersInteractions_Issue95(t *testing.T) {
	store, deps := setupToolsTest(t)

	d1, err := store.CreateDomain("L_owner", "d1", "", models.KnowledgeSpace{
		Concepts: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("create d1: %v", err)
	}
	d2, err := store.CreateDomain("L_owner", "d2", "", models.KnowledgeSpace{
		Concepts: []string{"x", "y"},
	})
	if err != nil {
		t.Fatalf("create d2: %v", err)
	}

	// Seed 5 interactions on D1's concept "a", each separated by 3h
	// so they are distinct sessions per the 2h session gap. None are
	// self-initiated → triggers the "no_initiative" pattern when the
	// handler considers learner-wide interactions.
	now := time.Now().UTC()
	for k := 0; k < 5; k++ {
		ts := now.Add(time.Duration(-k*3) * time.Hour)
		_, err := store.RawDB().Exec(
			`INSERT INTO interactions (learner_id, concept, activity_type, success, response_time, confidence, error_type, notes, hints_requested, self_initiated, calibration_id, is_proactive_review, misconception_type, misconception_detail, domain_id, bkt_slip, bkt_guess, created_at)
			 VALUES (?, ?, 'RECALL_EXERCISE', 1, 60, 0.5, '', '', 0, 0, '', 0, NULL, NULL, ?, NULL, NULL, ?)`,
			"L_owner", "a", d1.ID, ts,
		)
		if err != nil {
			t.Fatalf("seed interaction %d: %v", k, err)
		}
	}

	// Sanity: empty domain_id (the existing learner-wide behaviour)
	// surfaces the pattern, confirming the seed is sufficient.
	resAll := callTool(t, deps, registerGetMetacognitiveMirror, "L_owner", "get_metacognitive_mirror", map[string]any{})
	if resAll.IsError {
		t.Fatalf("learner-wide call errored: %q", resultText(resAll))
	}
	all := decodeResult(t, resAll)
	if all["mirror"] == nil {
		t.Fatalf("learner-wide call should surface the pattern (precondition for the test): got mirror=nil, payload=%+v", all)
	}

	// Scoped to D2: no interactions on D2 concepts → mirror must be nil.
	resD2 := callTool(t, deps, registerGetMetacognitiveMirror, "L_owner", "get_metacognitive_mirror", map[string]any{
		"domain_id": d2.ID,
	})
	if resD2.IsError {
		t.Fatalf("d2-scoped call errored: %q", resultText(resD2))
	}
	d2out := decodeResult(t, resD2)
	if d2out["mirror"] != nil {
		t.Fatalf("issue #95: domain_id=%s must scope to D2's empty concept set, expected mirror=nil, got %+v", d2.ID, d2out["mirror"])
	}
}

// TestGetMetacognitiveMirror_UnknownDomainIDRejected_Issue95 asserts that
// passing a domain_id that doesn't belong to the learner returns the
// canonical "domain not found" error, matching the behaviour of every
// other tool that resolves a domain (e.g. get_pending_alerts).
func TestGetMetacognitiveMirror_UnknownDomainIDRejected_Issue95(t *testing.T) {
	store, deps := setupToolsTest(t)
	// Create a domain owned by L_attacker. From L_owner's perspective
	// this id is foreign and resolveDomain must refuse.
	foreign, err := store.CreateDomain("L_attacker", "secret", "", models.KnowledgeSpace{
		Concepts: []string{"z"},
	})
	if err != nil {
		t.Fatalf("create foreign domain: %v", err)
	}
	res := callTool(t, deps, registerGetMetacognitiveMirror, "L_owner", "get_metacognitive_mirror", map[string]any{
		"domain_id": foreign.ID,
	})
	if !res.IsError {
		t.Fatalf("expected IsError on foreign domain_id, got payload=%q", resultText(res))
	}
	if got := resultText(res); got != "domain not found" {
		t.Fatalf("expected error %q, got %q", "domain not found", got)
	}
}

// TestGetMetacognitiveMirror_EmptyDomainIDStaysLearnerWide is the
// regression guard for the contract that an empty domain_id keeps the
// existing learner-wide behaviour (so callers that don't pass anything
// don't see a behaviour change).
func TestGetMetacognitiveMirror_EmptyDomainIDStaysLearnerWide(t *testing.T) {
	store, deps := setupToolsTest(t)

	d1, err := store.CreateDomain("L_owner", "d1", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	})
	if err != nil {
		t.Fatalf("create d1: %v", err)
	}
	now := time.Now().UTC()
	for k := 0; k < 5; k++ {
		ts := now.Add(time.Duration(-k*3) * time.Hour)
		_, err := store.RawDB().Exec(
			`INSERT INTO interactions (learner_id, concept, activity_type, success, response_time, confidence, error_type, notes, hints_requested, self_initiated, calibration_id, is_proactive_review, misconception_type, misconception_detail, domain_id, bkt_slip, bkt_guess, created_at)
			 VALUES (?, 'a', 'RECALL_EXERCISE', 1, 60, 0.5, '', '', 0, 0, '', 0, NULL, NULL, ?, NULL, NULL, ?)`,
			"L_owner", d1.ID, ts,
		)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	res := callTool(t, deps, registerGetMetacognitiveMirror, "L_owner", "get_metacognitive_mirror", map[string]any{})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["mirror"] == nil {
		t.Fatalf("empty domain_id must keep learner-wide behaviour, expected mirror!=nil, got %+v", out)
	}
}
