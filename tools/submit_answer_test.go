// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"testing"

	"tutor-mcp/models"
)

func TestSubmitAnswer_Correct_HappyPath(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callToolWithSampling(t, deps, registerSubmitAnswer, "L_owner",
		"submit_answer",
		map[string]any{
			"answer":        "42",
			"concept":       "a",
			"activity_type": "PRACTICE",
			"domain_id":     d.ID,
		},
		`{"correct": true, "explanation": "bien vu"}`,
	)
	if res.IsError {
		t.Fatalf("submit_answer errored: %s", resultText(res))
	}
	out := decodeStructured(t, res)
	if out["screen"] != "feedback" {
		t.Fatalf("expected screen=feedback, got %v", out["screen"])
	}
	if out["correct"] != true {
		t.Fatalf("expected correct=true, got %v", out["correct"])
	}
	if out["explanation"] != "bien vu" {
		t.Fatalf("expected explanation=bien vu, got %v", out["explanation"])
	}

	// Side effect: an interaction was recorded for concept "a".
	ints, _ := store.GetRecentInteractionsByLearner("L_owner", 5)
	found := false
	for _, i := range ints {
		if i.Concept == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an interaction recorded for concept 'a', got %v", ints)
	}

	// The cognitive state was updated by BKT/FSRS/IRT — verify the
	// ConceptState's PMastery moved away from the seeded 0.30.
	gotCS, _ := store.GetConceptState("L_owner", "a")
	if gotCS == nil {
		t.Fatalf("expected ConceptState for 'a' after submit_answer")
	}
	if gotCS.PMastery == 0.30 {
		t.Fatalf("expected PMastery to change after BKT update on correct=true, still 0.30")
	}
}

func TestSubmitAnswer_MalformedThenRecover_Retry(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callToolWithSamplingSeq(t, deps, registerSubmitAnswer, "L_owner",
		"submit_answer",
		map[string]any{"answer": "x", "concept": "a", "activity_type": "PRACTICE", "domain_id": d.ID},
		[]string{
			"This is not JSON at all.",
			`{"correct": false, "explanation": "essaie encore", "error_type": "wrong"}`,
		},
	)
	if res.IsError {
		t.Fatalf("submit_answer errored: %s", resultText(res))
	}
	out := decodeStructured(t, res)
	if out["screen"] != "feedback" {
		t.Fatalf("expected screen=feedback, got %v", out["screen"])
	}
	if out["correct"] != false {
		t.Fatalf("expected correct=false (after retry), got %v", out["correct"])
	}
	if out["explanation"] != "essaie encore" {
		t.Fatalf("explanation: %v", out["explanation"])
	}
}

func TestSubmitAnswer_MalformedTwice_FallbackB(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callToolWithSamplingSeq(t, deps, registerSubmitAnswer, "L_owner",
		"submit_answer",
		map[string]any{"answer": "x", "concept": "a", "activity_type": "PRACTICE", "domain_id": d.ID},
		[]string{"junk1", "junk2"},
	)
	if res.IsError {
		t.Fatalf("submit_answer errored: %s", resultText(res))
	}
	out := decodeStructured(t, res)
	if out["mode"] != "fallback_b" {
		t.Fatalf("expected mode=fallback_b after 2 malformed, got %v", out["mode"])
	}
	if out["parsed_failed"] != true {
		t.Fatalf("expected parsed_failed=true, got %v", out["parsed_failed"])
	}
}

