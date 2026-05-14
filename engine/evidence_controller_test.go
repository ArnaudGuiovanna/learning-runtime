// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"strings"
	"testing"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"
)

func TestApplyEvidenceController_WeakEvidenceAvoidsMasteryChallenge(t *testing.T) {
	in := baseEvidenceControllerInput()
	in.Activity.Type = models.ActivityMasteryChallenge
	in.Activity.Format = "build_challenge"
	in.TransferProfile.ReadinessLabel = TransferReadinessReady
	in.EvidenceQuality.Quality = EvidenceQualityWeak

	got := ApplyEvidenceController(in)
	if !got.Adjusted {
		t.Fatalf("expected adjustment")
	}
	if got.Activity.Type != models.ActivityPractice {
		t.Fatalf("type: got %q, want PRACTICE", got.Activity.Type)
	}
	if strings.Contains(got.Activity.PromptForLLM, "Target difficulty:") {
		t.Fatalf("prompt should not embed stale numeric difficulty: %q", got.Activity.PromptForLLM)
	}
}

func TestApplyEvidenceController_UnobservedTransferPrefersProbe(t *testing.T) {
	in := baseEvidenceControllerInput()
	in.TransferProfile.ReadinessLabel = TransferReadinessUnobserved

	got := ApplyEvidenceController(in)
	if !got.Adjusted {
		t.Fatalf("expected adjustment")
	}
	if got.Activity.Type != models.ActivityTransferProbe {
		t.Fatalf("type: got %q, want TRANSFER_PROBE", got.Activity.Type)
	}
}

func TestApplyEvidenceController_BlockedTransferPrefersFeynman(t *testing.T) {
	in := baseEvidenceControllerInput()
	in.TransferProfile.ReadinessLabel = TransferReadinessBlocked

	got := ApplyEvidenceController(in)
	if !got.Adjusted {
		t.Fatalf("expected adjustment")
	}
	if got.Activity.Type != models.ActivityFeynmanPrompt {
		t.Fatalf("type: got %q, want FEYNMAN_PROMPT", got.Activity.Type)
	}
}

func TestApplyEvidenceController_StrongEvidenceKeepsActivity(t *testing.T) {
	in := baseEvidenceControllerInput()
	in.TransferProfile.ReadinessLabel = TransferReadinessReady
	in.EvidenceQuality.Quality = EvidenceQualityStrong

	got := ApplyEvidenceController(in)
	if got.Adjusted {
		t.Fatalf("expected no adjustment, got %+v", got)
	}
	if got.Activity.Type != in.Activity.Type {
		t.Fatalf("type changed: got %q, want %q", got.Activity.Type, in.Activity.Type)
	}
}

func baseEvidenceControllerInput() EvidenceControllerInput {
	return EvidenceControllerInput{
		Activity: models.Activity{
			Type:             models.ActivityMasteryChallenge,
			Concept:          "goroutines",
			DifficultyTarget: 0.75,
			Format:           "build_challenge",
			EstimatedMinutes: 45,
			Rationale:        "[phase=MAINTENANCE] selected",
			PromptForLLM:     BuildActivityPrompt(models.ActivityMasteryChallenge, "goroutines", "build_challenge"),
		},
		ConceptState: &models.ConceptState{
			Concept:  "goroutines",
			PMastery: algorithms.MasteryBKT() + 0.05,
		},
		EvidenceQuality: EvidenceQualityAssessment{Quality: EvidenceQualityStrong},
	}
}
