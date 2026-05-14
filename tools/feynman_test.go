// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"strings"
	"testing"

	"tutor-mcp/models"
)

func TestFeynmanChallenge_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerFeynmanChallenge, "", "feynman_challenge", map[string]any{"concept": "x"})
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}

func TestFeynmanChallenge_MissingConcept(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerFeynmanChallenge, "L_owner", "feynman_challenge", map[string]any{"concept": ""})
	if !res.IsError || !strings.Contains(resultText(res), "concept is required") {
		t.Fatalf("got %q", resultText(res))
	}
}

func TestFeynmanChallenge_NotFound(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerFeynmanChallenge, "L_owner", "feynman_challenge", map[string]any{"concept": "ghost"})
	if !res.IsError || !strings.Contains(resultText(res), "concept not found") {
		t.Fatalf("got %q", resultText(res))
	}
}

func TestFeynmanChallenge_NotMastered(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "calc")
	cs.PMastery = 0.4
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callTool(t, deps, registerFeynmanChallenge, "L_owner", "feynman_challenge", map[string]any{"concept": "calc"})
	if res.IsError {
		t.Fatalf("expected non-error result, got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["eligible"] != false {
		t.Fatalf("expected eligible=false, got %v", out)
	}
}

func TestFeynmanChallenge_EligibleHappyPath(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "calc")
	cs.PMastery = 0.95
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callTool(t, deps, registerFeynmanChallenge, "L_owner", "feynman_challenge", map[string]any{"concept": "calc"})
	if res.IsError {
		t.Fatalf("expected success, got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["eligible"] != true {
		t.Fatalf("expected eligible=true, got %v", out)
	}
	if out["concept"] != "calc" {
		t.Fatalf("concept mismatch: %v", out["concept"])
	}
	if _, ok := out["concept_id"]; ok {
		t.Fatalf("did not expect legacy concept_id alias in result: %v", out)
	}
}

func TestFeynmanChallenge_AcceptsLegacyConceptID(t *testing.T) {
	store, deps := setupToolsTest(t)
	cs := models.NewConceptState("L_owner", "legacy_calc")
	cs.PMastery = 0.95
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callTool(t, deps, registerFeynmanChallenge, "L_owner", "feynman_challenge", map[string]any{"concept_id": "legacy_calc"})
	if res.IsError {
		t.Fatalf("expected success, got %q", resultText(res))
	}
	out := decodeResult(t, res)
	if out["concept"] != "legacy_calc" {
		t.Fatalf("expected canonical concept key, got %v", out)
	}
	if _, ok := out["concept_id"]; ok {
		t.Fatalf("did not expect legacy concept_id alias in result: %v", out)
	}
}
