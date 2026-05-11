// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"tutor-mcp/models"
)

const (
	// DefaultEvidenceRecentWindow is the recency horizon used when callers do
	// not need to tune evidence freshness. The value is deliberately explicit:
	// evidence analysis is pure and never calls time.Now().
	DefaultEvidenceRecentWindow = 14 * 24 * time.Hour
)

type EvidenceProfile struct {
	LearnerID             string   `json:"learner_id"`
	Concept               string   `json:"concept"`
	Count                 int      `json:"count"`
	RecentCount           int      `json:"recent_count"`
	ActivityTypeDiversity int      `json:"activity_type_diversity"`
	HasRecall             bool     `json:"has_recall"`
	HasPractice           bool     `json:"has_practice"`
	HasFeynman            bool     `json:"has_feynman"`
	HasTransfer           bool     `json:"has_transfer"`
	HasMastery            bool     `json:"has_mastery"`
	SuccessfulTypes       []string `json:"successful_types"`
	FailureTypes          []string `json:"failure_types"`
	RubricPresentCount    int      `json:"rubric_present_count"`
	RubricPresenceRatio   float64  `json:"rubric_presence_ratio"`
}

type EvidenceQuality string

const (
	EvidenceQualityWeak     EvidenceQuality = "weak"
	EvidenceQualityModerate EvidenceQuality = "moderate"
	EvidenceQualityStrong   EvidenceQuality = "strong"
)

type EvidenceQualityAssessment struct {
	Quality EvidenceQuality `json:"quality"`
	Reasons []string        `json:"reasons"`
}

// BuildEvidenceProfile summarizes all interactions matching learnerID and
// concept using DefaultEvidenceRecentWindow. The function is pure: callers must
// provide the observation time used to decide recency.
func BuildEvidenceProfile(learnerID, concept string, interactions []*models.Interaction, now time.Time) EvidenceProfile {
	return BuildEvidenceProfileWithWindow(learnerID, concept, interactions, now, DefaultEvidenceRecentWindow)
}

// BuildEvidenceProfileWithWindow summarizes interaction evidence for one
// learner/concept pair. RecentCount is zero when now is zero or recentWindow is
// non-positive, so tests and callers do not inherit hidden clock behavior.
func BuildEvidenceProfileWithWindow(learnerID, concept string, interactions []*models.Interaction, now time.Time, recentWindow time.Duration) EvidenceProfile {
	profile := EvidenceProfile{
		LearnerID: learnerID,
		Concept:   concept,
	}

	activityTypes := map[string]struct{}{}
	successfulTypes := map[string]struct{}{}
	failureTypes := map[string]struct{}{}

	var cutoff time.Time
	countRecency := !now.IsZero() && recentWindow > 0
	if countRecency {
		cutoff = now.Add(-recentWindow)
	}

	for _, interaction := range interactions {
		if interaction == nil {
			continue
		}
		if interaction.LearnerID != learnerID || interaction.Concept != concept {
			continue
		}

		profile.Count++
		if countRecency && !interaction.CreatedAt.IsZero() && !interaction.CreatedAt.Before(cutoff) {
			profile.RecentCount++
		}
		if hasRubricEvidence(interaction) {
			profile.RubricPresentCount++
		}

		activityType := strings.TrimSpace(interaction.ActivityType)
		if activityType == "" {
			continue
		}
		activityTypes[activityType] = struct{}{}
		markEvidenceFlags(activityType, &profile)
		if interaction.Success {
			successfulTypes[activityType] = struct{}{}
		} else {
			failureTypes[activityType] = struct{}{}
		}
	}

	profile.ActivityTypeDiversity = len(activityTypes)
	profile.SuccessfulTypes = sortedKeys(successfulTypes)
	profile.FailureTypes = sortedKeys(failureTypes)
	if profile.Count > 0 {
		profile.RubricPresenceRatio = float64(profile.RubricPresentCount) / float64(profile.Count)
	}

	return profile
}

// MasteryEvidenceQuality grades whether a mastery claim is backed by enough
// varied, recent, successful evidence. It does not inspect BKT state; it only
// evaluates the shape of the evidence profile.
func MasteryEvidenceQuality(profile EvidenceProfile) EvidenceQualityAssessment {
	successfulKinds := successfulEvidenceKinds(profile)
	successfulDiversity := len(profile.SuccessfulTypes)
	hasMasterySuccess := successfulKinds[evidenceKindMastery]
	hasDeepSuccess := successfulKinds[evidenceKindFeynman] || successfulKinds[evidenceKindTransfer]

	var quality EvidenceQuality
	switch {
	case profile.Count >= 5 &&
		profile.RecentCount >= 2 &&
		successfulDiversity >= 3 &&
		hasMasterySuccess &&
		hasDeepSuccess:
		quality = EvidenceQualityStrong
	case profile.Count >= 3 &&
		profile.RecentCount >= 1 &&
		successfulDiversity >= 2:
		quality = EvidenceQualityModerate
	default:
		quality = EvidenceQualityWeak
	}

	return EvidenceQualityAssessment{
		Quality: quality,
		Reasons: evidenceQualityReasons(profile, quality, successfulKinds),
	}
}

const (
	evidenceKindRecall   = "recall"
	evidenceKindPractice = "practice"
	evidenceKindFeynman  = "feynman"
	evidenceKindTransfer = "transfer"
	evidenceKindMastery  = "mastery"
)

func markEvidenceFlags(activityType string, profile *EvidenceProfile) {
	switch evidenceKind(activityType) {
	case evidenceKindRecall:
		profile.HasRecall = true
	case evidenceKindPractice:
		profile.HasPractice = true
	case evidenceKindFeynman:
		profile.HasFeynman = true
	case evidenceKindTransfer:
		profile.HasTransfer = true
	case evidenceKindMastery:
		profile.HasMastery = true
	}
}

func successfulEvidenceKinds(profile EvidenceProfile) map[string]bool {
	kinds := map[string]bool{}
	for _, activityType := range profile.SuccessfulTypes {
		if kind := evidenceKind(activityType); kind != "" {
			kinds[kind] = true
		}
	}
	return kinds
}

func evidenceKind(activityType string) string {
	switch strings.TrimSpace(activityType) {
	case string(models.ActivityRecall), "RECALL":
		return evidenceKindRecall
	case string(models.ActivityPractice):
		return evidenceKindPractice
	case string(models.ActivityFeynmanPrompt):
		return evidenceKindFeynman
	case string(models.ActivityTransferProbe):
		return evidenceKindTransfer
	case string(models.ActivityMasteryChallenge):
		return evidenceKindMastery
	default:
		return ""
	}
}

func hasRubricEvidence(interaction *models.Interaction) bool {
	return strings.TrimSpace(interaction.RubricJSON) != "" ||
		strings.TrimSpace(interaction.RubricScoreJSON) != ""
}

func evidenceQualityReasons(profile EvidenceProfile, quality EvidenceQuality, successfulKinds map[string]bool) []string {
	reasons := []string{
		fmt.Sprintf("%d total interactions, %d recent", profile.Count, profile.RecentCount),
		fmt.Sprintf("%d distinct activity types, %d successful activity types", profile.ActivityTypeDiversity, len(profile.SuccessfulTypes)),
	}

	switch {
	case profile.RubricPresentCount > 0:
		reasons = append(reasons, fmt.Sprintf("rubric evidence present on %d/%d interactions (%.0f%%)",
			profile.RubricPresentCount, profile.Count, profile.RubricPresenceRatio*100))
	case profile.Count > 0:
		reasons = append(reasons, "no rubric evidence present")
	}

	if len(profile.FailureTypes) > 0 {
		reasons = append(reasons, "failures observed in: "+strings.Join(profile.FailureTypes, ", "))
	}

	switch quality {
	case EvidenceQualityStrong:
		reasons = append(reasons, "successful mastery evidence plus successful feynman or transfer evidence")
	case EvidenceQualityModerate:
		reasons = appendMissingStrongReasons(reasons, profile, successfulKinds)
	default:
		reasons = appendMissingModerateReasons(reasons, profile)
	}

	return reasons
}

func appendMissingStrongReasons(reasons []string, profile EvidenceProfile, successfulKinds map[string]bool) []string {
	if profile.Count < 5 {
		reasons = append(reasons, "strong evidence needs at least 5 interactions")
	}
	if profile.RecentCount < 2 {
		reasons = append(reasons, "strong evidence needs at least 2 recent interactions")
	}
	if len(profile.SuccessfulTypes) < 3 {
		reasons = append(reasons, "strong evidence needs at least 3 successful activity types")
	}
	if !successfulKinds[evidenceKindMastery] {
		reasons = append(reasons, "strong evidence needs a successful mastery challenge")
	}
	if !successfulKinds[evidenceKindFeynman] && !successfulKinds[evidenceKindTransfer] {
		reasons = append(reasons, "strong evidence needs successful feynman or transfer evidence")
	}
	return reasons
}

func appendMissingModerateReasons(reasons []string, profile EvidenceProfile) []string {
	if profile.Count < 3 {
		reasons = append(reasons, "moderate evidence needs at least 3 interactions")
	}
	if profile.RecentCount < 1 {
		reasons = append(reasons, "moderate evidence needs at least 1 recent interaction")
	}
	if len(profile.SuccessfulTypes) < 2 {
		reasons = append(reasons, "moderate evidence needs at least 2 successful activity types")
	}
	return reasons
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
