// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"strings"
	"testing"

	"tutor-mcp/models"
)

func TestRecordInteraction_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerRecordInteraction, "", "record_interaction", map[string]any{
		"concept":               "x",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.8,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestRecordInteraction_MissingConcept(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.8,
		"notes":                 "",
	})
	if !res.IsError || !strings.Contains(resultText(res), "concept is required") {
		t.Fatalf("got %q", resultText(res))
	}
}

func TestRecordInteraction_HappyPath_Success(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math") // concepts: ["a","b"]

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 12.0,
		"confidence":            0.9,
		"notes":                 "great",
		"hints_requested":       1,
		"self_initiated":        true,
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["updated"] != true {
		t.Fatalf("expected updated=true, got %v", out)
	}
	if _, ok := out["new_mastery"]; !ok {
		t.Fatalf("expected new_mastery key, got %v", out)
	}
	if out["engagement_signal"] != "positive" {
		t.Fatalf("expected positive engagement (success+conf>=0.8), got %v", out["engagement_signal"])
	}

	// DB: interaction created.
	recents, err := store.GetRecentInteractionsByLearner("L_owner", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recents) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(recents))
	}
	if !recents[0].Success || recents[0].Concept != "a" {
		t.Fatalf("unexpected interaction: %+v", recents[0])
	}

	// DB: concept state upserted.
	cs, err := store.GetConceptState("L_owner", "a")
	if err != nil {
		t.Fatalf("expected concept state: %v", err)
	}
	if cs.Reps == 0 {
		t.Fatalf("expected reps to be incremented, got %d", cs.Reps)
	}
}

func TestRecordInteraction_FailureDecliningSignal(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               false,
		"response_time_seconds": 30.0,
		"confidence":            0.1,
		"notes":                 "",
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["engagement_signal"] != "declining" {
		t.Fatalf("expected declining engagement, got %v", out["engagement_signal"])
	}
}

func TestRecordInteraction_StoresMisconceptionOnFailure(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               false,
		"response_time_seconds": 20.0,
		"confidence":            0.4,
		"notes":                 "",
		"misconception_type":    "off_by_one",
		"misconception_detail":  "uses < instead of <=",
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}

	recents, _ := store.GetRecentInteractionsByLearner("L_owner", 5)
	if len(recents) == 0 {
		t.Fatal("no interactions recorded")
	}
	got := recents[0]
	if got.MisconceptionType != "off_by_one" {
		t.Fatalf("misconception not stored: %+v", got)
	}
}

func TestRecordInteraction_MisconceptionIgnoredOnSuccess(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.7,
		"notes":                 "",
		"misconception_type":    "off_by_one",
		"misconception_detail":  "ignored",
	})
	if res.IsError {
		t.Fatalf("got %q", resultText(res))
	}

	recents, _ := store.GetRecentInteractionsByLearner("L_owner", 5)
	if len(recents) == 0 {
		t.Fatal("no interactions")
	}
	if recents[0].MisconceptionType != "" {
		t.Fatalf("misconception should NOT be stored on success: %+v", recents[0])
	}
}

func TestRecordInteraction_RejectsUnknownConcept(t *testing.T) {
	store, deps := setupToolsTest(t)
	// makeOwnerDomain creates a domain with concepts ["a","b"].
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "ghost",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.8,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected error for unknown concept, got %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "ghost") {
		t.Fatalf("expected error to mention the unknown concept name, got %q", resultText(res))
	}

	// And no orphan ConceptState row should have been created.
	if cs, err := store.GetConceptState("L_owner", "ghost"); err == nil && cs != nil {
		t.Fatalf("orphan concept_state row created for unknown concept: %+v", cs)
	}
}

func TestRecordInteraction_NoActiveDomain(t *testing.T) {
	_, deps := setupToolsTest(t)
	// L_owner has no domain at all — record_interaction must signal
	// needs_domain_setup (issue #33: uniform shape across chat-side tools).
	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "anything",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.8,
		"notes":                 "",
	})
	out := decodeResult(t, res)
	if got, _ := out["needs_domain_setup"].(bool); !got {
		t.Fatalf("expected needs_domain_setup=true, got %v (raw %q)", out, resultText(res))
	}
}

func TestRecordInteraction_RejectsOutOfRangeConfidence(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            2.5, // out-of-range; legal interval is [0,1]
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected error for confidence=2.5, got %q", resultText(res))
	}
	msg := resultText(res)
	if !strings.Contains(msg, "confidence") {
		t.Fatalf("expected error message to mention 'confidence', got %q", msg)
	}

	// And nothing should have been written to the cognitive store.
	recents, _ := store.GetRecentInteractionsByLearner("L_owner", 5)
	if len(recents) != 0 {
		t.Fatalf("expected no interactions persisted, got %d", len(recents))
	}
}

func TestRecordInteraction_RejectsNegativeResponseTime(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": -30.0,
		"confidence":            0.5,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected error for response_time_seconds=-30, got %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "response_time_seconds") {
		t.Fatalf("expected error to mention 'response_time_seconds', got %q", resultText(res))
	}
}

func TestRecordInteraction_RejectsOutOfRangeHints(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.5,
		"hints_requested":       9999,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected error for hints_requested=9999, got %q", resultText(res))
	}
	if !strings.Contains(resultText(res), "hints_requested") {
		t.Fatalf("expected error to mention 'hints_requested', got %q", resultText(res))
	}
}

func TestComputeCognitiveSignals(t *testing.T) {
	// Less than 3 interactions → no signals.
	fatigue, frust := computeCognitiveSignals([]*models.Interaction{
		{Success: true, Confidence: 0.8, ResponseTime: 10},
	})
	if fatigue != "none" || frust != "none" {
		t.Fatalf("expected none/none for tiny session, got %s/%s", fatigue, frust)
	}

	// Build a frustration scenario: consecutive failures + low confidence.
	bad := []*models.Interaction{
		{Success: false, Confidence: 0.1, ResponseTime: 20},
		{Success: false, Confidence: 0.2, ResponseTime: 22},
		{Success: false, Confidence: 0.1, ResponseTime: 25},
		{Success: true, Confidence: 0.5, ResponseTime: 10},
	}
	_, frust2 := computeCognitiveSignals(bad)
	if frust2 == "none" {
		t.Fatalf("expected non-none frustration, got %q", frust2)
	}

	// Build fatigue: poor recent vs solid earlier window.
	long := []*models.Interaction{
		// recent (newest first) — poor
		{Success: false, Confidence: 0.3, ResponseTime: 60},
		{Success: false, Confidence: 0.3, ResponseTime: 50},
		{Success: false, Confidence: 0.3, ResponseTime: 40},
		{Success: true, Confidence: 0.4, ResponseTime: 30},
		{Success: false, Confidence: 0.3, ResponseTime: 30},
		// earlier window — strong
		{Success: true, Confidence: 0.9, ResponseTime: 5},
		{Success: true, Confidence: 0.9, ResponseTime: 5},
		{Success: true, Confidence: 0.9, ResponseTime: 5},
		{Success: true, Confidence: 0.9, ResponseTime: 5},
		{Success: true, Confidence: 0.9, ResponseTime: 5},
	}
	fatigue3, _ := computeCognitiveSignals(long)
	if fatigue3 == "none" {
		t.Fatalf("expected fatigue signal, got %q", fatigue3)
	}
}
