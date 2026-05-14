// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"fmt"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"
)

type EvidenceControllerInput struct {
	Activity           models.Activity
	ConceptState       *models.ConceptState
	EvidenceQuality    EvidenceQualityAssessment
	MasteryUncertainty MasteryUncertainty
	TransferProfile    TransferProfile
}

type EvidenceControllerDecision struct {
	Activity  models.Activity
	Adjusted  bool
	Rationale string
}

func ApplyEvidenceController(input EvidenceControllerInput) EvidenceControllerDecision {
	activity := input.Activity
	if !evidenceControllerEligible(activity) || input.ConceptState == nil {
		return EvidenceControllerDecision{Activity: activity}
	}
	if input.ConceptState.PMastery < algorithms.MasteryBKT() {
		return EvidenceControllerDecision{Activity: activity}
	}

	switch input.TransferProfile.ReadinessLabel {
	case TransferReadinessBlocked:
		return evidenceAdjusted(activity, models.ActivityFeynmanPrompt, "transfer_repair", 15,
			"evidence controller: transfer blocked -> Feynman explanation before further challenge")
	case TransferReadinessUnobserved:
		return evidenceAdjusted(activity, models.ActivityTransferProbe, "transfer_novel_context", 20,
			"evidence controller: high mastery but transfer unobserved -> transfer probe")
	}

	if activity.Type == models.ActivityMasteryChallenge && input.EvidenceQuality.Quality == EvidenceQualityWeak {
		return evidenceAdjusted(activity, models.ActivityPractice, "varied_evidence_probe", 12,
			"evidence controller: weak mastery evidence -> varied proof before mastery challenge")
	}

	return EvidenceControllerDecision{Activity: activity}
}

func evidenceControllerEligible(activity models.Activity) bool {
	switch activity.Type {
	case models.ActivityCloseSession, models.ActivityRest, models.ActivitySetupDomain:
		return false
	default:
		return activity.Concept != ""
	}
}

func evidenceAdjusted(activity models.Activity, t models.ActivityType, format string, minutes int, rationale string) EvidenceControllerDecision {
	activity.Type = t
	activity.Format = format
	activity.EstimatedMinutes = minutes
	activity.Rationale = fmt.Sprintf("%s; %s", activity.Rationale, rationale)
	activity.PromptForLLM = BuildActivityPrompt(activity.Type, activity.Concept, activity.Format)
	return EvidenceControllerDecision{Activity: activity, Adjusted: true, Rationale: rationale}
}

func BuildActivityPrompt(t models.ActivityType, concept, format string) string {
	return fmt.Sprintf("Generate a %s activity on %s. Format: %s. Use the final structured activity.difficulty_target field as the difficulty target.", t, concept, format)
}
