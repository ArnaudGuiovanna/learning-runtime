package engine

import (
	"testing"
	"time"

	"tutor-mcp/models"
)

func ptrTime(t time.Time) *time.Time { return &t }

func TestComputeAlertsForgetting(t *testing.T) {
	states := []*models.ConceptState{
		{Concept: "goroutines", Stability: 0.2, ElapsedDays: 5, PMastery: 0.5, CardState: "review",
			LastReview: ptrTime(time.Now().AddDate(0, 0, -5))},
	}
	alerts := ComputeAlerts(states, nil, time.Time{})
	found := false
	for _, a := range alerts {
		if a.Type == models.AlertForgetting && a.Concept == "goroutines" {
			found = true
			if a.Urgency != models.UrgencyCritical && a.Urgency != models.UrgencyWarning {
				t.Errorf("urgency = %s, want critical or warning", a.Urgency)
			}
		}
	}
	if !found {
		t.Error("expected FORGETTING alert for goroutines")
	}
}

func TestComputeAlertsMasteryReady(t *testing.T) {
	// Stability=1.0, ElapsedDays=1 → retention ≈ 0.90 (well above 0.40), so the
	// FORGETTING/MASTERY_READY arbitration introduced for sub-issue #54 does not
	// suppress MASTERY_READY here.
	states := []*models.ConceptState{{Concept: "basics", PMastery: 0.90, Stability: 1.0, ElapsedDays: 1, CardState: "review"}}
	alerts := ComputeAlerts(states, nil, time.Time{})
	found := false
	for _, a := range alerts {
		if a.Type == models.AlertMasteryReady && a.Concept == "basics" {
			found = true
		}
	}
	if !found {
		t.Error("expected MASTERY_READY for basics")
	}
}

// TestComputeAlertsMasteryForgettingArbitration covers the four corners of the
// (PMastery × retention) matrix to verify the alert-level arbitration rule:
// FORGETTING at UrgencyCritical (retention < 0.30) suppresses MASTERY_READY for
// the same concept. FORGETTING at UrgencyWarning (0.30 ≤ retention < 0.40) does
// NOT suppress — the learner is in a nuanced "almost mastered, slightly stale"
// state that warrants both nudges.
//
// Threshold rationale: retention < 0.30 corresponds to the existing
// UrgencyCritical band defined in ComputeAlerts (engine/alert.go). Retuning the
// suppression cutoff means changing both that constant and the comment in
// ComputeAlerts together.
func TestComputeAlertsMasteryForgettingArbitration(t *testing.T) {
	// Retention closed-form: (1 + (19/81)*elapsed/stability)^(-0.5)
	//   elapsed=1,  stability=1.0  → retention ≈ 0.900 (>= 0.40)
	//   elapsed=5,  stability=0.2  → retention ≈ 0.382 (in [0.30, 0.40))
	//   elapsed=5,  stability=0.1  → retention ≈ 0.280 (< 0.30)
	masteryHigh := 0.90 // >= MasteryBKT() (0.85)
	masteryLow := 0.50  // <  MasteryBKT()

	cases := []struct {
		name             string
		pMastery         float64
		stability        float64
		elapsedDays      int
		wantForgetting   bool
		wantForgettingUrgency models.AlertUrgency
		wantMasteryReady bool
	}{
		{
			name:             "high mastery + high retention → only MASTERY_READY",
			pMastery:         masteryHigh,
			stability:        1.0,
			elapsedDays:      1,
			wantForgetting:   false,
			wantMasteryReady: true,
		},
		{
			name:                  "high mastery + warning retention → both alerts kept",
			pMastery:              masteryHigh,
			stability:             0.2,
			elapsedDays:           5,
			wantForgetting:        true,
			wantForgettingUrgency: models.UrgencyWarning,
			wantMasteryReady:      true,
		},
		{
			name:                  "high mastery + critical retention → only FORGETTING (arbitration)",
			pMastery:              masteryHigh,
			stability:             0.1,
			elapsedDays:           5,
			wantForgetting:        true,
			wantForgettingUrgency: models.UrgencyCritical,
			wantMasteryReady:      false,
		},
		{
			name:                  "low mastery + critical retention → only FORGETTING",
			pMastery:              masteryLow,
			stability:             0.1,
			elapsedDays:           5,
			wantForgetting:        true,
			wantForgettingUrgency: models.UrgencyCritical,
			wantMasteryReady:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			states := []*models.ConceptState{
				{
					Concept:     "arbitration_concept",
					Stability:   tc.stability,
					ElapsedDays: tc.elapsedDays,
					PMastery:    tc.pMastery,
					CardState:   "review",
				},
			}
			alerts := ComputeAlerts(states, nil, time.Time{})

			gotForgetting := false
			gotForgettingUrgency := models.AlertUrgency("")
			gotMasteryReady := false
			for _, a := range alerts {
				if a.Concept != "arbitration_concept" {
					continue
				}
				switch a.Type {
				case models.AlertForgetting:
					gotForgetting = true
					gotForgettingUrgency = a.Urgency
				case models.AlertMasteryReady:
					gotMasteryReady = true
				}
			}

			if gotForgetting != tc.wantForgetting {
				t.Errorf("FORGETTING presence: got %v, want %v", gotForgetting, tc.wantForgetting)
			}
			if tc.wantForgetting && gotForgettingUrgency != tc.wantForgettingUrgency {
				t.Errorf("FORGETTING urgency: got %s, want %s", gotForgettingUrgency, tc.wantForgettingUrgency)
			}
			if gotMasteryReady != tc.wantMasteryReady {
				t.Errorf("MASTERY_READY presence: got %v, want %v", gotMasteryReady, tc.wantMasteryReady)
			}
		})
	}
}

func TestComputeAlertsZPDDrift(t *testing.T) {
	interactions := []*models.Interaction{
		{Concept: "pointers", Success: false},
		{Concept: "pointers", Success: false},
		{Concept: "pointers", Success: false},
	}
	states := []*models.ConceptState{{Concept: "pointers", PMastery: 0.3, CardState: "learning"}}
	alerts := ComputeAlerts(states, interactions, time.Time{})
	found := false
	for _, a := range alerts {
		if a.Type == models.AlertZPDDrift && a.Concept == "pointers" {
			found = true
		}
	}
	if !found {
		t.Error("expected ZPD_DRIFT for pointers")
	}
}

func TestComputeAlertsIRTPredictiveZPDDrift(t *testing.T) {
	// Concept with low theta and high difficulty → IRT pCorrect < 0.55
	// Theta = -1.0, FSRS difficulty = 8.0 → IRT difficulty ≈ 1.67
	// pCorrect = 1/(1+exp(-1*(-1.0-1.67))) ≈ 0.065 — well below 0.55
	states := []*models.ConceptState{
		{Concept: "channels", Theta: -1.0, Difficulty: 8.0, Reps: 3, CardState: "review"},
	}
	alerts := ComputeAlerts(states, nil, time.Time{})

	found := false
	for _, a := range alerts {
		if a.Type == models.AlertZPDDrift && a.Concept == "channels" && a.Urgency == models.UrgencyInfo {
			found = true
		}
	}
	if !found {
		t.Error("expected IRT-predictive ZPD_DRIFT (info) for channels")
	}
}

func TestComputeAlertsIRTPredictiveSkipsWhenFailureBased(t *testing.T) {
	// Concept already has 3 failures → failure-based ZPD_DRIFT (warning) exists.
	// IRT-predictive should NOT add a duplicate.
	states := []*models.ConceptState{
		{Concept: "pointers", Theta: -1.0, Difficulty: 8.0, Reps: 5, CardState: "review", PMastery: 0.3},
	}
	interactions := []*models.Interaction{
		{Concept: "pointers", Success: false},
		{Concept: "pointers", Success: false},
		{Concept: "pointers", Success: false},
	}
	alerts := ComputeAlerts(states, interactions, time.Time{})

	zpdCount := 0
	for _, a := range alerts {
		if a.Type == models.AlertZPDDrift && a.Concept == "pointers" {
			zpdCount++
		}
	}
	if zpdCount != 1 {
		t.Errorf("expected exactly 1 ZPD_DRIFT for pointers, got %d", zpdCount)
	}
}

func TestComputeAlertsDependencyIncreasing(t *testing.T) {
	autonomyScores := []float64{0.4, 0.5, 0.6} // declining (newest first)
	alerts := ComputeMetacognitiveAlerts(autonomyScores, 0.3, nil, nil)

	found := false
	for _, a := range alerts {
		if a.Type == models.AlertDependencyIncreasing {
			found = true
		}
	}
	if !found {
		t.Error("expected DEPENDENCY_INCREASING alert")
	}
}

func TestComputeAlertsCalibrationDiverging(t *testing.T) {
	alerts := ComputeMetacognitiveAlerts(nil, 1.6, nil, nil)

	found := false
	for _, a := range alerts {
		if a.Type == models.AlertCalibrationDiverging {
			found = true
		}
	}
	if !found {
		t.Error("expected CALIBRATION_DIVERGING alert")
	}
}

func TestComputeAlertsAffectNegative(t *testing.T) {
	affects := []*models.AffectState{
		{Satisfaction: 2},
		{Satisfaction: 1},
	}
	alerts := ComputeMetacognitiveAlerts(nil, 0, affects, nil)

	found := false
	for _, a := range alerts {
		if a.Type == models.AlertAffectNegative {
			found = true
		}
	}
	if !found {
		t.Error("expected AFFECT_NEGATIVE alert")
	}
}

func TestComputeAlertsTransferBlocked(t *testing.T) {
	states := []*models.ConceptState{
		{Concept: "A", PMastery: 0.90},
	}
	transfers := []*models.TransferRecord{
		{ConceptID: "A", ContextType: "real_world", Score: 0.3},
		{ConceptID: "A", ContextType: "interview", Score: 0.4},
	}
	alerts := ComputeMetacognitiveAlerts(nil, 0, nil, nil, WithTransferData(states, transfers))

	found := false
	for _, a := range alerts {
		if a.Type == models.AlertTransferBlocked {
			found = true
		}
	}
	if !found {
		t.Error("expected TRANSFER_BLOCKED alert")
	}
}
