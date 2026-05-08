// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"strings"
	"testing"
)

// Issue #31 reproducers: chat-side tools must reject pathologically long
// free-text payloads at the boundary instead of silently bloating the
// interactions / learners / affects tables. Each test posts a payload
// that exceeds the field's cap and expects an errorResult.

func TestRecordInteraction_RejectsOversizedNotes(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.5,
		"notes":                 strings.Repeat("x", maxNoteLen+1),
	})
	if !res.IsError {
		t.Fatalf("expected length-cap rejection, got %q", resultText(res))
	}
}

func TestRecordInteraction_RejectsOversizedConcept(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               strings.Repeat("c", maxShortLabelLen+1),
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 5.0,
		"confidence":            0.5,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected length-cap rejection, got %q", resultText(res))
	}
}

func TestUpdateLearnerProfile_RejectsOversizedBackground(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerUpdateLearnerProfile, "L_owner", "update_learner_profile", map[string]any{
		"background": strings.Repeat("b", maxNoteLen+1),
	})
	if !res.IsError {
		t.Fatalf("expected length-cap rejection, got %q", resultText(res))
	}
}

func TestRecordAffect_RejectsOversizedSessionID(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerRecordAffect, "L_owner", "record_affect", map[string]any{
		"session_id": strings.Repeat("s", maxShortLabelLen+1),
	})
	if !res.IsError {
		t.Fatalf("expected length-cap rejection, got %q", resultText(res))
	}
}

func TestValidateString_PureBoundary(t *testing.T) {
	if err := validateString("f", "abc", 3); err != nil {
		t.Errorf("equal-to-max should pass, got %v", err)
	}
	if err := validateString("f", "abcd", 3); err == nil {
		t.Error("over-max should fail")
	}
	if err := validateString("f", "", 3); err != nil {
		t.Errorf("empty must always pass, got %v", err)
	}
}
