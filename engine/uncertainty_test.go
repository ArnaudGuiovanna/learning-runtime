// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"testing"
	"time"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"
)

func uncertaintyConceptState(concept string, mastery float64, reps int, at time.Time) *models.ConceptState {
	cs := models.NewConceptState("L1", concept)
	cs.CardState = "review"
	cs.PMastery = mastery
	cs.Reps = reps
	cs.LastReview = &at
	cs.UpdatedAt = at
	return cs
}

func uncertaintyInteraction(concept, activityType string, success bool, at time.Time) *models.Interaction {
	return &models.Interaction{
		LearnerID:    "L1",
		Concept:      concept,
		ActivityType: activityType,
		Success:      success,
		CreatedAt:    at,
	}
}

func hasUncertaintyReason(got MasteryUncertainty, reason MasteryUncertaintyReason) bool {
	for _, r := range got.Reasons {
		if r == reason {
			return true
		}
	}
	return false
}

func TestComputeMasteryUncertainty_FewObservations(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	cs := uncertaintyConceptState("closures", 0.40, 1, now.Add(-24*time.Hour))
	interactions := []*models.Interaction{
		uncertaintyInteraction("closures", string(models.ActivityRecall), true, now.Add(-time.Hour)),
	}

	got := ComputeMasteryUncertainty(cs, interactions, MasteryEvidenceProfile{Now: now})

	if !hasUncertaintyReason(got, UncertaintyReasonFewObservations) {
		t.Fatalf("expected few observations reason, got %+v", got)
	}
	if got.ConfidenceLabel != MasteryConfidenceLow {
		t.Fatalf("expected low confidence, got %+v", got)
	}
	if got.UncertaintyScore < 0 || got.UncertaintyScore > 1 {
		t.Fatalf("score out of range: %.3f", got.UncertaintyScore)
	}
}

func TestComputeMasteryUncertainty_ManyFreshEvidenceIsHighConfidence(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	cs := uncertaintyConceptState("interfaces", 0.96, 6, now.Add(-6*time.Hour))
	interactions := []*models.Interaction{
		uncertaintyInteraction("interfaces", string(models.ActivityRecall), true, now.Add(-time.Hour)),
		uncertaintyInteraction("interfaces", string(models.ActivityDebuggingCase), true, now.Add(-2*time.Hour)),
		uncertaintyInteraction("interfaces", string(models.ActivityMasteryChallenge), true, now.Add(-3*time.Hour)),
		uncertaintyInteraction("interfaces", string(models.ActivityRecall), true, now.Add(-4*time.Hour)),
		uncertaintyInteraction("interfaces", string(models.ActivityDebuggingCase), true, now.Add(-5*time.Hour)),
		uncertaintyInteraction("interfaces", string(models.ActivityMasteryChallenge), true, now.Add(-6*time.Hour)),
	}

	got := ComputeMasteryUncertainty(cs, interactions, MasteryEvidenceProfile{Now: now})

	if got.ConfidenceLabel != MasteryConfidenceHigh {
		t.Fatalf("expected high confidence, got %+v", got)
	}
	if got.UncertaintyScore > 0.10 {
		t.Fatalf("expected very low uncertainty, got %.3f (%+v)", got.UncertaintyScore, got.Reasons)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("expected no reasons for fresh, diverse successes, got %+v", got.Reasons)
	}
}

func TestComputeMasteryUncertainty_ModelNearThreshold(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	cs := uncertaintyConceptState("generics", algorithms.MasteryBKT()-0.01, 5, now.Add(-2*time.Hour))
	interactions := []*models.Interaction{
		uncertaintyInteraction("generics", string(models.ActivityRecall), true, now.Add(-time.Hour)),
		uncertaintyInteraction("generics", string(models.ActivityDebuggingCase), true, now.Add(-2*time.Hour)),
		uncertaintyInteraction("generics", string(models.ActivityMasteryChallenge), true, now.Add(-3*time.Hour)),
		uncertaintyInteraction("generics", string(models.ActivityRecall), true, now.Add(-4*time.Hour)),
	}

	got := ComputeMasteryUncertainty(cs, interactions, MasteryEvidenceProfile{Now: now})

	if !hasUncertaintyReason(got, UncertaintyReasonModelNearThreshold) {
		t.Fatalf("expected near-threshold reason, got %+v", got)
	}
	if got.ConfidenceLabel != MasteryConfidenceMedium {
		t.Fatalf("expected medium confidence near threshold, got %+v", got)
	}
}

func TestComputeMasteryUncertainty_RecentErrors(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	cs := uncertaintyConceptState("goroutines", 0.60, 8, now.Add(-time.Hour))
	interactions := []*models.Interaction{
		uncertaintyInteraction("goroutines", string(models.ActivityRecall), false, now.Add(-time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityDebuggingCase), false, now.Add(-2*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityRecall), false, now.Add(-3*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityDebuggingCase), true, now.Add(-4*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityRecall), true, now.Add(-72*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityDebuggingCase), true, now.Add(-73*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityMasteryChallenge), true, now.Add(-74*time.Hour)),
		uncertaintyInteraction("goroutines", string(models.ActivityRecall), true, now.Add(-75*time.Hour)),
	}
	profile := MasteryEvidenceProfile{
		Now:               now,
		RecentErrorWindow: 24 * time.Hour,
	}

	got := ComputeMasteryUncertainty(cs, interactions, profile)

	if !hasUncertaintyReason(got, UncertaintyReasonHighErrorRate) {
		t.Fatalf("expected high error rate reason, got %+v", got)
	}
	if got.ConfidenceLabel != MasteryConfidenceMedium {
		t.Fatalf("expected medium confidence for recent mixed errors, got %+v", got)
	}
}
