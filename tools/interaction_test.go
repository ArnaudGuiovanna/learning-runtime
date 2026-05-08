// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"math"
	"strings"
	"testing"
	"time"

	"tutor-mcp/algorithms"
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

// Issue #24: domain_id parameter must be honored — i.e. resolved (not silently
// ignored) and persisted on the interaction row so audits can tell apart
// progress on the same concept name across two domains.
func TestRecordInteraction_DomainIDIsPersistedOnRow(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"domain_id":             d.ID,
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 4.0,
		"confidence":            0.7,
		"notes":                 "",
	})
	if res.IsError {
		t.Fatalf("expected success, got %q", resultText(res))
	}

	recents, _ := store.GetRecentInteractionsByLearner("L_owner", 1)
	if len(recents) != 1 {
		t.Fatalf("expected 1 interaction row, got %d", len(recents))
	}
	if got := recents[0].DomainID; got != d.ID {
		t.Errorf("DomainID = %q, want %q", got, d.ID)
	}
}

// Issue #24: an explicit domain_id pointing at someone else's domain must be
// rejected — concept membership validation runs against the resolved domain
// and a foreign domain has no overlap with the learner's concept set.
func TestRecordInteraction_DomainIDRejectsForeignDomain(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")
	foreign, err := store.CreateDomain("L_other", "shared", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	})
	if err != nil {
		t.Fatalf("create foreign domain: %v", err)
	}

	res := callTool(t, deps, registerRecordInteraction, "L_owner", "record_interaction", map[string]any{
		"concept":               "a",
		"domain_id":             foreign.ID,
		"activity_type":         "RECALL_EXERCISE",
		"success":               true,
		"response_time_seconds": 4.0,
		"confidence":            0.7,
		"notes":                 "",
	})
	if !res.IsError {
		t.Fatalf("expected errorResult on foreign domain_id, got %q", resultText(res))
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

// Issue #53: applyInteraction's BKT → FSRS → IRT chain must be commutative
// in the sense that the IRT step reads the *prior* (pre-FSRS) Difficulty,
// not the value FSRS just overwrote. The non-commutative read is on
// cs.Difficulty: FSRS rewrites it, then IRT consumes it via
// FSRSDifficultyToIRT to compute the θ update. Reading post-FSRS difficulty
// silently shifts θ along the difficulty curve, biasing the learner's IRT
// estimate by the size of the difficulty step. Snapshot the read-only
// inputs at the top, run the three updates against the snapshot, write
// once at the end.
func TestApplyInteraction_IRTReadsPreFSRSDifficulty(t *testing.T) {
	store, deps := setupToolsTest(t)
	makeOwnerDomain(t, store, "L_owner", "math")

	// Seed a concept state in the Review FSRS state with a non-default
	// Difficulty and Reps. Difficulty=9.0 sits at the high end of the
	// FSRS [1,10] range; on an Again rating FSRS's nextDifficulty pulls
	// it sharply down (toward ~6.9), so post-FSRS difficulty differs
	// from pre-FSRS by ~2 difficulty points. With a failure response
	// IRT pushes θ negative, and the two starting difficulties produce
	// distinguishable θ values inside the [-4, 4] clamp window.
	now := time.Now().UTC()
	last := now.Add(-48 * time.Hour)
	seed := &models.ConceptState{
		LearnerID:     "L_owner",
		Concept:       "a",
		Stability:     5.0,
		Difficulty:    9.0,
		ElapsedDays:   2,
		ScheduledDays: 5,
		Reps:          5,
		Lapses:        0,
		CardState:     "review",
		LastReview:    &last,
		PMastery:      0.4,
		PLearn:        0.15,
		PForget:       0.05,
		PSlip:         0.1,
		PGuess:        0.2,
		// θ seed is non-zero and close to the pre-FSRS IRT difficulty so
		// the Newton iterations in IRTUpdateTheta land *inside* the
		// [-4, 4] clamp window, leaving room to distinguish the pre-FSRS
		// vs post-FSRS difficulty cases.
		Theta: 2.0,
	}
	if err := store.UpsertConceptState(seed); err != nil {
		t.Fatalf("seed concept state: %v", err)
	}

	// Compute the expected θ update by running IRT against the *pre-FSRS*
	// difficulty (9.0). success=false maps to FSRS rating Again.
	expectedItem := algorithms.IRTItem{
		Difficulty:     algorithms.FSRSDifficultyToIRT(9.0),
		Discrimination: 1.0,
	}
	expectedTheta := algorithms.IRTUpdateTheta(seed.Theta, []algorithms.IRTItem{expectedItem}, []bool{false})

	// And the *post-FSRS* difficulty θ — what today's buggy code computes.
	// We capture this so the assertion can also confirm the two values are
	// in fact distinguishable on this scenario (otherwise the test is
	// vacuous).
	postFSRSDiff := algorithms.ReviewCard(algorithms.FSRSCard{
		Stability:     5.0,
		Difficulty:    9.0,
		ElapsedDays:   2,
		ScheduledDays: 5,
		Reps:          5,
		Lapses:        0,
		State:         algorithms.Review,
		LastReview:    last,
	}, algorithms.Again, now).Difficulty
	buggyItem := algorithms.IRTItem{
		Difficulty:     algorithms.FSRSDifficultyToIRT(postFSRSDiff),
		Discrimination: 1.0,
	}
	buggyTheta := algorithms.IRTUpdateTheta(seed.Theta, []algorithms.IRTItem{buggyItem}, []bool{false})
	if math.Abs(expectedTheta-buggyTheta) < 1e-6 {
		t.Fatalf("test setup is vacuous: pre/post FSRS thetas are identical (%v)", expectedTheta)
	}

	cs, err := applyInteraction(deps, "L_owner", interactionInput{
		Concept:             "a",
		ActivityType:        "RECALL_EXERCISE",
		Success:             false, // → FSRS rating Again
		ResponseTimeSeconds: 10,
		Confidence:          0.4,
	}, now)
	if err != nil {
		t.Fatalf("applyInteraction: %v", err)
	}

	if math.Abs(cs.Theta-expectedTheta) > 1e-9 {
		t.Fatalf("IRT consumed post-FSRS difficulty: got θ=%v, want θ=%v (post-FSRS-buggy θ=%v)",
			cs.Theta, expectedTheta, buggyTheta)
	}

	// Sanity: FSRS *did* rewrite Difficulty and Reps, so the read-then-
	// update concern is real on this path.
	if cs.Difficulty == 9.0 {
		t.Fatalf("FSRS did not rewrite Difficulty as expected (still 9.0)")
	}
	if cs.Reps != 6 {
		t.Fatalf("FSRS did not increment Reps as expected: got %d, want 6", cs.Reps)
	}
}
