// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"strings"
	"testing"

	"tutor-mcp/models"
)

func TestLearningNegotiation_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerLearningNegotiation, "", "learning_negotiation", map[string]any{
		"session_id": "s1",
	})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestLearningNegotiation_NoDomain(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerLearningNegotiation, "L_owner", "learning_negotiation", map[string]any{
		"session_id": "s1",
	})
	// Issue #33: a missing domain (no DomainID supplied) returns the
	// uniform needs_domain_setup jsonResult — not an errorResult — so the
	// LLM can branch consistently across all chat-side tools.
	out := decodeResult(t, res)
	if got, _ := out["needs_domain_setup"].(bool); !got {
		t.Fatalf("expected needs_domain_setup=true, got %v", out)
	}
}

func TestLearningNegotiation_SystemPlanOnly(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerLearningNegotiation, "L_owner", "learning_negotiation", map[string]any{
		"session_id": "s1",
		"domain_id":  d.ID,
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if _, ok := out["system_plan"]; !ok {
		t.Fatalf("expected system_plan, got %v", out)
	}
	if _, ok := out["learner_proposal"]; ok {
		t.Fatalf("learner_proposal should not be present, got %v", out["learner_proposal"])
	}
}

// TestLearningNegotiation_UnknownConceptRejected guards issue #92: a learner
// (or hallucinating LLM) can pass a concept name that does not exist in the
// active domain's Graph.Concepts. Without the validateConceptInDomain guard
// the negotiation tool silently builds a plan around the non-existent concept
// and returns accepted=true even though no prereqs exist. Mirror the
// record_interaction / transfer_challenge guard pattern.
func TestLearningNegotiation_UnknownConceptRejected(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math") // domain has {a, b}

	res := callTool(t, deps, registerLearningNegotiation, "L_owner", "learning_negotiation", map[string]any{
		"session_id":      "s1",
		"learner_concept": "ghost",
		"domain_id":       d.ID,
	})
	if !res.IsError {
		t.Fatalf("expected error for unknown concept, got %q", resultText(res))
	}
	msg := resultText(res)
	if !strings.Contains(msg, "ghost") || !strings.Contains(msg, "not part of domain") {
		t.Fatalf("expected error mentioning unknown concept and domain, got %q", msg)
	}
}

func TestLearningNegotiation_LearnerProposalAccepted(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")

	// Seed concept state for a (learner's choice).
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.5
	cs.Difficulty = 5.0
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callTool(t, deps, registerLearningNegotiation, "L_owner", "learning_negotiation", map[string]any{
		"session_id":        "s1",
		"learner_concept":   "a",
		"learner_rationale": "envie",
		"domain_id":         d.ID,
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["learner_proposal"] != "a" {
		t.Fatalf("expected learner_proposal=a, got %v", out["learner_proposal"])
	}
	if _, ok := out["accepted"]; !ok {
		t.Fatalf("expected accepted key, got %v", out)
	}
	if _, ok := out["accepted_plan"]; !ok {
		t.Fatalf("expected accepted_plan key, got %v", out)
	}
	if _, ok := out["counts_as_self_initiated"]; !ok {
		t.Fatalf("expected counts_as_self_initiated key, got %v", out)
	}
}
