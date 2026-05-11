// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"strings"
	"testing"

	"tutor-mcp/models"
)

func TestCheckMastery_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerCheckMastery, "", "check_mastery", map[string]any{"concept": "x"})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestCheckMastery_MissingConcept(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerCheckMastery, "L_owner", "check_mastery", map[string]any{"concept": ""})
	if !res.IsError || !strings.Contains(resultText(res), "concept is required") {
		t.Fatalf("got %q", resultText(res))
	}
}

func TestCheckMastery_NotFound(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerCheckMastery, "L_owner", "check_mastery", map[string]any{"concept": "ghost"})
	if !res.IsError || !strings.Contains(resultText(res), "concept state not found") {
		t.Fatalf("got %q", resultText(res))
	}
}

func TestCheckMastery_NotReady(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "calc")
	cs.PMastery = 0.4
	if err := store.InsertConceptStateIfNotExists(cs); err != nil {
		t.Fatal(err)
	}

	res := callTool(t, deps, registerCheckMastery, "L_owner", "check_mastery", map[string]any{"concept": "calc"})
	if res.IsError {
		t.Fatalf("did not expect error for low mastery, got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["mastery_ready"] != false {
		t.Fatalf("expected mastery_ready=false, got %v", out["mastery_ready"])
	}
}

func TestCheckMastery_Ready(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "calc")
	cs.PMastery = 0.95
	if err := store.InsertConceptStateIfNotExists(cs); err != nil {
		t.Fatal(err)
	}
	// InsertConceptStateIfNotExists does not update if exists. Use Upsert to set mastery.
	if err := store.UpsertConceptState(cs); err != nil {
		t.Fatal(err)
	}
	for _, activityType := range []models.ActivityType{
		models.ActivityRecall,
		models.ActivityPractice,
		models.ActivityMasteryChallenge,
	} {
		if err := store.CreateInteraction(&models.Interaction{
			LearnerID:    "L_owner",
			Concept:      "calc",
			ActivityType: string(activityType),
			Success:      true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	res := callTool(t, deps, registerCheckMastery, "L_owner", "check_mastery", map[string]any{"concept": "calc"})
	if res.IsError {
		t.Fatalf("expected success: %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["mastery_ready"] != true {
		t.Fatalf("expected mastery_ready=true, got %v", out["mastery_ready"])
	}
}

func TestCheckMastery_HighBKTWeakEvidenceNotReady(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "calc")
	cs.PMastery = 0.95
	if err := store.UpsertConceptState(cs); err != nil {
		t.Fatal(err)
	}

	res := callTool(t, deps, registerCheckMastery, "L_owner", "check_mastery", map[string]any{"concept": "calc"})
	if res.IsError {
		t.Fatalf("expected success: %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["mastery_ready"] != false {
		t.Fatalf("expected mastery_ready=false with weak evidence, got %v", out["mastery_ready"])
	}
	if out["bkt_mastery_ready"] != true {
		t.Fatalf("expected bkt_mastery_ready=true, got %v", out["bkt_mastery_ready"])
	}
	if _, ok := out["evidence_quality"].(map[string]any); !ok {
		t.Fatalf("expected evidence_quality in result, got %v", out)
	}
}
