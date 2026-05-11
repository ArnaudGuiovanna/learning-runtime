package engine

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"tutor-mcp/models"
)

func TestBuildEvidenceProfileCountsDiversityRubricAndFilters(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	interactions := []*models.Interaction{
		evidenceInteraction("L1", "C1", string(models.ActivityRecall), true, now.Add(-24*time.Hour), "", `{"overall":1}`),
		evidenceInteraction("L1", "C1", string(models.ActivityPractice), false, now.Add(-2*24*time.Hour), "", ""),
		evidenceInteraction("L1", "C1", string(models.ActivityMasteryChallenge), true, now.Add(-20*24*time.Hour), "", ""),
		evidenceInteraction("L1", "C1", string(models.ActivityFeynmanPrompt), true, now.Add(-30*time.Minute), `{"criteria":[]}`, ""),
		evidenceInteraction("L1", "C1", string(models.ActivityTransferProbe), false, now.Add(-1*time.Hour), "", ""),
		evidenceInteraction("L2", "C1", string(models.ActivityTransferProbe), true, now.Add(-1*time.Hour), `{"ignored":true}`, ""),
		evidenceInteraction("L1", "C2", string(models.ActivityTransferProbe), true, now.Add(-1*time.Hour), `{"ignored":true}`, ""),
		nil,
	}

	got := BuildEvidenceProfile("L1", "C1", interactions, now)

	if got.Count != 5 {
		t.Fatalf("Count = %d, want 5", got.Count)
	}
	if got.RecentCount != 4 {
		t.Fatalf("RecentCount = %d, want 4", got.RecentCount)
	}
	if got.ActivityTypeDiversity != 5 {
		t.Fatalf("ActivityTypeDiversity = %d, want 5", got.ActivityTypeDiversity)
	}
	if !got.HasRecall || !got.HasPractice || !got.HasFeynman || !got.HasTransfer || !got.HasMastery {
		t.Fatalf("expected all evidence flags true, got %+v", got)
	}

	wantSuccessfulTypes := []string{
		string(models.ActivityFeynmanPrompt),
		string(models.ActivityMasteryChallenge),
		string(models.ActivityRecall),
	}
	if !reflect.DeepEqual(got.SuccessfulTypes, wantSuccessfulTypes) {
		t.Fatalf("SuccessfulTypes = %v, want %v", got.SuccessfulTypes, wantSuccessfulTypes)
	}

	wantFailureTypes := []string{
		string(models.ActivityPractice),
		string(models.ActivityTransferProbe),
	}
	if !reflect.DeepEqual(got.FailureTypes, wantFailureTypes) {
		t.Fatalf("FailureTypes = %v, want %v", got.FailureTypes, wantFailureTypes)
	}

	if got.RubricPresentCount != 2 {
		t.Fatalf("RubricPresentCount = %d, want 2", got.RubricPresentCount)
	}
	if math.Abs(got.RubricPresenceRatio-0.4) > 0.0001 {
		t.Fatalf("RubricPresenceRatio = %.3f, want 0.400", got.RubricPresenceRatio)
	}
}

func TestBuildEvidenceProfileWithWindowUsesExplicitRecencyCutoff(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	interactions := []*models.Interaction{
		evidenceInteraction("L1", "C1", string(models.ActivityRecall), true, now.Add(-48*time.Hour), "", ""),
		evidenceInteraction("L1", "C1", string(models.ActivityPractice), true, now.Add(-48*time.Hour-time.Nanosecond), "", ""),
		evidenceInteraction("L1", "C1", string(models.ActivityFeynmanPrompt), true, now, "", ""),
	}

	got := BuildEvidenceProfileWithWindow("L1", "C1", interactions, now, 48*time.Hour)

	if got.Count != 3 {
		t.Fatalf("Count = %d, want 3", got.Count)
	}
	if got.RecentCount != 2 {
		t.Fatalf("RecentCount = %d, want 2", got.RecentCount)
	}
}

func TestBuildEvidenceProfileClassifiesLegacyRecall(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	interactions := []*models.Interaction{
		evidenceInteraction("L1", "C1", "RECALL", true, now, "", ""),
	}

	got := BuildEvidenceProfile("L1", "C1", interactions, now)

	if !got.HasRecall {
		t.Fatalf("HasRecall = false, want true for legacy RECALL activity")
	}
	if !reflect.DeepEqual(got.SuccessfulTypes, []string{"RECALL"}) {
		t.Fatalf("SuccessfulTypes = %v, want [RECALL]", got.SuccessfulTypes)
	}
}

func TestBuildEvidenceProfileZeroTimeDisablesRecentCount(t *testing.T) {
	interactions := []*models.Interaction{
		evidenceInteraction("L1", "C1", string(models.ActivityRecall), true, time.Now().UTC(), "", ""),
	}

	got := BuildEvidenceProfile("L1", "C1", interactions, time.Time{})

	if got.RecentCount != 0 {
		t.Fatalf("RecentCount = %d, want 0 when now is zero", got.RecentCount)
	}
}

func TestMasteryEvidenceQuality(t *testing.T) {
	tests := []struct {
		name             string
		profile          EvidenceProfile
		wantQuality      EvidenceQuality
		reasonSubstrings []string
	}{
		{
			name: "strong needs recent varied successful mastery and deep evidence",
			profile: EvidenceProfile{
				Count:                 6,
				RecentCount:           3,
				ActivityTypeDiversity: 5,
				SuccessfulTypes: []string{
					string(models.ActivityPractice),
					string(models.ActivityMasteryChallenge),
					string(models.ActivityTransferProbe),
				},
				RubricPresentCount:  4,
				RubricPresenceRatio: 4.0 / 6.0,
			},
			wantQuality: EvidenceQualityStrong,
			reasonSubstrings: []string{
				"successful mastery evidence",
				"rubric evidence present",
			},
		},
		{
			name: "moderate has some recent successful diversity but misses strong criteria",
			profile: EvidenceProfile{
				Count:                 3,
				RecentCount:           1,
				ActivityTypeDiversity: 2,
				SuccessfulTypes: []string{
					string(models.ActivityPractice),
					string(models.ActivityRecall),
				},
			},
			wantQuality: EvidenceQualityModerate,
			reasonSubstrings: []string{
				"strong evidence needs at least 5 interactions",
				"strong evidence needs a successful mastery challenge",
			},
		},
		{
			name: "weak when evidence is stale despite successful diversity",
			profile: EvidenceProfile{
				Count:                 4,
				RecentCount:           0,
				ActivityTypeDiversity: 2,
				SuccessfulTypes: []string{
					string(models.ActivityPractice),
					string(models.ActivityRecall),
				},
			},
			wantQuality: EvidenceQualityWeak,
			reasonSubstrings: []string{
				"moderate evidence needs at least 1 recent interaction",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MasteryEvidenceQuality(tc.profile)
			if got.Quality != tc.wantQuality {
				t.Fatalf("Quality = %q, want %q; reasons=%v", got.Quality, tc.wantQuality, got.Reasons)
			}
			for _, substring := range tc.reasonSubstrings {
				if !hasReasonContaining(got.Reasons, substring) {
					t.Fatalf("expected a reason containing %q, got %v", substring, got.Reasons)
				}
			}
		})
	}
}

func evidenceInteraction(learnerID, concept, activityType string, success bool, createdAt time.Time, rubricJSON, rubricScoreJSON string) *models.Interaction {
	return &models.Interaction{
		LearnerID:       learnerID,
		Concept:         concept,
		ActivityType:    activityType,
		Success:         success,
		RubricJSON:      rubricJSON,
		RubricScoreJSON: rubricScoreJSON,
		CreatedAt:       createdAt,
	}
}

func hasReasonContaining(reasons []string, substring string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, substring) {
			return true
		}
	}
	return false
}
