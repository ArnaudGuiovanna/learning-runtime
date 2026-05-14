// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"math"
	"testing"
	"time"

	"tutor-mcp/models"
)

func TestBuildDecisionReplaySummary_AggregatesFindings(t *testing.T) {
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	snapshots := []*models.PedagogicalSnapshot{
		replayTestSnapshot(1, 101, "fractions", string(models.ActivityPractice),
			`{"p_mastery":0.40}`,
			`{"rubric":{"criteria":["correctness"]},"rubric_score":{"overall":0.8}}`,
			`{"p_mastery":0.55}`,
			now),
		replayTestSnapshot(2, 102, "fractions", string(models.ActivityMasteryChallenge),
			`{"p_mastery":0.55}`,
			`{}`,
			`{"p_mastery":0.90}`,
			now.Add(time.Minute)),
		replayTestSnapshot(3, 103, "algebra", string(models.ActivityPractice),
			`{"p_mastery":0.80}`,
			`{"rubric":{"criteria":["reasoning"]},"rubric_score":{"overall":0.4}}`,
			`{"p_mastery":0.70}`,
			now.Add(2*time.Minute)),
		replayTestSnapshot(4, 104, "pause", string(models.ActivityRest),
			`{}`,
			`{}`,
			`{}`,
			now.Add(3*time.Minute)),
	}
	interactions := []*models.Interaction{
		replayTestInteraction(101, "fractions", string(models.ActivityPractice), now),
		replayTestInteraction(102, "fractions", string(models.ActivityMasteryChallenge), now.Add(time.Minute)),
	}
	interactions[0].RubricJSON = `{"criteria":["correctness"]}`
	interactions[0].RubricScoreJSON = `{"overall":0.8}`
	snapshots[1].InterpretationBrief = "The learner may be overfitting the worked example."

	summary := BuildDecisionReplaySummary(snapshots, interactions)

	if summary.TotalSnapshots != 4 {
		t.Fatalf("TotalSnapshots = %d, want 4", summary.TotalSnapshots)
	}
	if summary.ActivityDistribution[string(models.ActivityPractice)] != 2 {
		t.Fatalf("practice distribution = %d, want 2", summary.ActivityDistribution[string(models.ActivityPractice)])
	}
	if summary.ActivityDistribution[string(models.ActivityMasteryChallenge)] != 1 {
		t.Fatalf("mastery distribution = %d, want 1", summary.ActivityDistribution[string(models.ActivityMasteryChallenge)])
	}
	if summary.MasteryDeltaSamples != 3 {
		t.Fatalf("MasteryDeltaSamples = %d, want 3", summary.MasteryDeltaSamples)
	}
	if math.Abs(summary.AverageMasteryDelta-((0.15+0.35-0.10)/3.0)) > 1e-9 {
		t.Fatalf("AverageMasteryDelta = %.12f", summary.AverageMasteryDelta)
	}
	if summary.SuspiciousJumpCount != 1 || summary.SuspiciousJumps[0].SnapshotID != 2 {
		t.Fatalf("unexpected suspicious jumps: %+v", summary.SuspiciousJumps)
	}
	if summary.SuspiciousJumps[0].InterpretationBrief != "The learner may be overfitting the worked example." {
		t.Fatalf("missing interpretation brief on suspicious jump: %+v", summary.SuspiciousJumps[0])
	}
	if summary.NegativeDeltaCount != 1 || summary.NegativeDeltas[0].SnapshotID != 3 {
		t.Fatalf("unexpected negative deltas: %+v", summary.NegativeDeltas)
	}
	if summary.MissingRubricEvidenceCount != 1 {
		t.Fatalf("MissingRubricEvidenceCount = %d, want 1: %+v", summary.MissingRubricEvidenceCount, summary.MissingRubricEvidence)
	}
	if got := summary.MissingRubricEvidence[0].Missing; len(got) != 2 || got[0] != "rubric" || got[1] != "rubric_score" {
		t.Fatalf("missing rubric pieces = %+v", got)
	}
	if summary.TransferAfterMasteryGapCount != 1 || summary.TransferAfterMasteryGaps[0].Concept != "fractions" {
		t.Fatalf("unexpected transfer gaps: %+v", summary.TransferAfterMasteryGaps)
	}
	if summary.TransferAfterMasteryGaps[0].MasteryInterpretationBrief != "The learner may be overfitting the worked example." {
		t.Fatalf("missing interpretation brief on transfer gap: %+v", summary.TransferAfterMasteryGaps[0])
	}
	if summary.SnapshotJSONIssueCount != 0 {
		t.Fatalf("unexpected JSON issues: %+v", summary.SnapshotJSONIssues)
	}
}

func TestBuildDecisionReplaySummary_RobustSnapshotJSON(t *testing.T) {
	now := time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC)
	snapshots := []*models.PedagogicalSnapshot{
		replayTestSnapshot(1, 101, "strings-ok", string(models.ActivityPractice),
			`{"p_mastery":"0.20"}`,
			`{"rubric":{},"rubric_score":{}}`,
			`{"p_mastery":"0.70"}`,
			now),
		replayTestSnapshot(2, 102, "bad-json", string(models.ActivityNewConcept),
			`{"p_mastery":`,
			`[]`,
			`{"p_mastery":"NaN"}`,
			now.Add(time.Minute)),
		replayTestSnapshot(3, 103, "out-of-range", string(models.ActivityNewConcept),
			`{"p_mastery":0.10}`,
			`{}`,
			`{"p_mastery":1.40}`,
			now.Add(2*time.Minute)),
	}

	summary := BuildDecisionReplaySummary(snapshots, nil)

	if summary.MasteryDeltaSamples != 1 {
		t.Fatalf("MasteryDeltaSamples = %d, want 1", summary.MasteryDeltaSamples)
	}
	if math.Abs(summary.AverageMasteryDelta-0.50) > 1e-9 {
		t.Fatalf("AverageMasteryDelta = %.12f, want 0.5", summary.AverageMasteryDelta)
	}
	if summary.SnapshotJSONIssueCount != 4 {
		t.Fatalf("SnapshotJSONIssueCount = %d, want 4: %+v", summary.SnapshotJSONIssueCount, summary.SnapshotJSONIssues)
	}
	fields := map[string]bool{}
	for _, issue := range summary.SnapshotJSONIssues {
		fields[issue.Field] = true
	}
	for _, field := range []string{"before_json", "observation_json", "after_json.p_mastery"} {
		if !fields[field] {
			t.Fatalf("expected issue for %s in %+v", field, summary.SnapshotJSONIssues)
		}
	}
}

func TestBuildDecisionReplaySummary_TransferAfterMasterySatisfiedByLaterInteraction(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	snapshots := []*models.PedagogicalSnapshot{
		replayTestSnapshot(1, 101, "limits", string(models.ActivityMasteryChallenge),
			`{"p_mastery":0.84}`,
			`{"rubric":{},"rubric_score":{}}`,
			`{"p_mastery":0.86}`,
			now),
	}
	interactions := []*models.Interaction{
		replayTestInteraction(201, "limits", string(models.ActivityTransferProbe), now.Add(time.Hour)),
	}

	summary := BuildDecisionReplaySummary(snapshots, interactions)

	if summary.TransferAfterMasteryGapCount != 0 {
		t.Fatalf("unexpected transfer gaps: %+v", summary.TransferAfterMasteryGaps)
	}
}

func replayTestSnapshot(id, interactionID int64, concept, activityType, beforeJSON, observationJSON, afterJSON string, createdAt time.Time) *models.PedagogicalSnapshot {
	return &models.PedagogicalSnapshot{
		ID:              id,
		InteractionID:   interactionID,
		LearnerID:       "L1",
		DomainID:        "D1",
		Concept:         concept,
		ActivityType:    activityType,
		BeforeJSON:      beforeJSON,
		ObservationJSON: observationJSON,
		AfterJSON:       afterJSON,
		DecisionJSON:    `{"source":"test"}`,
		CreatedAt:       createdAt,
	}
}

func replayTestInteraction(id int64, concept, activityType string, createdAt time.Time) *models.Interaction {
	return &models.Interaction{
		ID:           id,
		LearnerID:    "L1",
		DomainID:     "D1",
		Concept:      concept,
		ActivityType: activityType,
		CreatedAt:    createdAt,
	}
}
