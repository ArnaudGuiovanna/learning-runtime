package algorithms

import (
	"math"
	"testing"
)

func TestBKTUpdateCorrect(t *testing.T) {
	state := BKTState{PMastery: 0.5, PLearn: 0.3, PForget: 0.05, PSlip: 0.1, PGuess: 0.2}
	updated := BKTUpdate(state, true)
	if !approxEqual(updated.PMastery, 0.8318, 0.01) {
		t.Errorf("PMastery after correct = %f, want ~0.8318", updated.PMastery)
	}
}

func TestBKTUpdateIncorrect(t *testing.T) {
	state := BKTState{PMastery: 0.5, PLearn: 0.3, PForget: 0.05, PSlip: 0.1, PGuess: 0.2}
	updated := BKTUpdate(state, false)
	if !approxEqual(updated.PMastery, 0.3722, 0.01) {
		t.Errorf("PMastery after incorrect = %f, want ~0.3722", updated.PMastery)
	}
}

func TestBKTMasteryThreshold(t *testing.T) {
	state := BKTState{PMastery: 0.86}
	if !BKTIsMastered(state) { t.Error("0.86 should be mastered") }
	state.PMastery = 0.84
	if BKTIsMastered(state) { t.Error("0.84 should NOT be mastered") }
}

func TestBKTConvergence(t *testing.T) {
	state := BKTState{PMastery: 0.1, PLearn: 0.3, PForget: 0.05, PSlip: 0.1, PGuess: 0.2}
	for i := 0; i < 10; i++ { state = BKTUpdate(state, true) }
	if state.PMastery < 0.85 { t.Errorf("after 10 correct, PMastery = %f, want >= 0.85", state.PMastery) }
}

func TestBKTUpdateClampsToOne(t *testing.T) {
	// Pre-clamp PMastery should remain in [0, 1].
	state := BKTState{PMastery: 0.99, PLearn: 0.99, PForget: 0.0, PSlip: 0.0, PGuess: 0.0}
	updated := BKTUpdate(state, true)
	if updated.PMastery < 0 || updated.PMastery > 1 {
		t.Errorf("PMastery out of [0,1]: %f", updated.PMastery)
	}
	if !approxEqual(updated.PMastery, 1.0, 1e-9) {
		t.Errorf("PMastery should clamp to 1, got %f", updated.PMastery)
	}
}

func TestBKTUpdateWithErrorType(t *testing.T) {
	base := BKTState{PMastery: 0.5, PLearn: 0.3, PForget: 0.05, PSlip: 0.1, PGuess: 0.2}

	tests := []struct {
		name      string
		correct   bool
		errorType string
		// expectation relative to plain BKTUpdate(base, correct)
		// for incorrect: SYNTAX_ERROR softens penalty (higher mastery),
		// KNOWLEDGE_GAP hardens penalty (lower mastery).
		// "" or unknown == standard.
		comparator string // "equal", "greater", "less"
	}{
		{name: "correct ignores errorType", correct: true, errorType: "SYNTAX_ERROR", comparator: "equal"},
		{name: "empty errorType uses standard", correct: false, errorType: "", comparator: "equal"},
		{name: "logic error uses standard", correct: false, errorType: "LOGIC_ERROR", comparator: "equal"},
		{name: "unknown errorType uses standard", correct: false, errorType: "FOOBAR", comparator: "equal"},
		{name: "syntax error softens penalty", correct: false, errorType: "SYNTAX_ERROR", comparator: "greater"},
		{name: "knowledge gap hardens penalty", correct: false, errorType: "KNOWLEDGE_GAP", comparator: "less"},
	}

	standard := BKTUpdate(base, false).PMastery
	standardCorrect := BKTUpdate(base, true).PMastery

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BKTUpdateWithErrorType(base, tc.correct, tc.errorType).PMastery
			ref := standard
			if tc.correct {
				ref = standardCorrect
			}
			switch tc.comparator {
			case "equal":
				if !approxEqual(got, ref, 1e-9) {
					t.Errorf("got %f, want %f (equal to standard)", got, ref)
				}
			case "greater":
				if got <= ref {
					t.Errorf("got %f, want > %f (softer penalty)", got, ref)
				}
			case "less":
				if got >= ref {
					t.Errorf("got %f, want < %f (harder penalty)", got, ref)
				}
			}
			if got < 0 || got > 1 {
				t.Errorf("PMastery out of [0,1]: %f", got)
			}
		})
	}
}

// TestBKTUpdateNoNaNOrInfOnDegenerateInputs verifies BKTUpdate never returns
// NaN or Inf when the input parameters make the marginal observation
// probability collapse to zero (e.g. PMastery=0 with PGuess=0 on a "correct"
// observation). Without a guard the Bayesian division 0/0 yields NaN, which
// then poisons every downstream consumer.
func TestBKTUpdateNoNaNOrInfOnDegenerateInputs(t *testing.T) {
	tests := []struct {
		name    string
		state   BKTState
		correct bool
	}{
		{
			// pCorrect = (1-0)*0 + 0*1 = 0  → 0/0 = NaN before the guard.
			name:    "correct with PMastery=0 and PGuess=0",
			state:   BKTState{PMastery: 0, PLearn: 0.3, PForget: 0.05, PSlip: 0, PGuess: 0},
			correct: true,
		},
		{
			// pCorrect = 0*1 + 1*0 = 0  → 0/0 = NaN before the guard.
			name:    "correct with PMastery=1 and PSlip=1, PGuess=0",
			state:   BKTState{PMastery: 1, PLearn: 0.3, PForget: 0.05, PSlip: 1, PGuess: 0},
			correct: true,
		},
		{
			// pIncorrect = 0*0 + 0*1 = 0  → 0/0 = NaN before the guard.
			name:    "incorrect with PMastery=0, PSlip=0, PGuess=1",
			state:   BKTState{PMastery: 0, PLearn: 0.3, PForget: 0.05, PSlip: 0, PGuess: 1},
			correct: false,
		},
		{
			// pIncorrect = 0*1 + 0*0 = 0  → 0/0 = NaN before the guard.
			name:    "incorrect with PMastery=1, PSlip=0, PGuess=1",
			state:   BKTState{PMastery: 1, PLearn: 0.3, PForget: 0.05, PSlip: 0, PGuess: 1},
			correct: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BKTUpdate(tc.state, tc.correct)
			if math.IsNaN(got.PMastery) || math.IsInf(got.PMastery, 0) {
				t.Errorf("PMastery is NaN/Inf: %v", got.PMastery)
			}
			if got.PMastery < 0 || got.PMastery > 1 {
				t.Errorf("PMastery out of [0,1]: %v", got.PMastery)
			}
		})
	}
}

// TestBKTUpdateWithErrorTypeNoNaNOrInf is the same guard at the
// BKTUpdateWithErrorType layer — KNOWLEDGE_GAP/SYNTAX_ERROR mutate inputs
// before delegating, so we want to make sure the guard still holds end-to-end.
func TestBKTUpdateWithErrorTypeNoNaNOrInf(t *testing.T) {
	degenerate := BKTState{PMastery: 0, PLearn: 0.3, PForget: 0.05, PSlip: 0, PGuess: 0}
	for _, et := range []string{"", "LOGIC_ERROR", "SYNTAX_ERROR", "KNOWLEDGE_GAP", "UNKNOWN"} {
		t.Run("errorType="+et, func(t *testing.T) {
			got := BKTUpdateWithErrorType(degenerate, true, et)
			if math.IsNaN(got.PMastery) || math.IsInf(got.PMastery, 0) {
				t.Errorf("PMastery NaN/Inf for errorType=%q: %v", et, got.PMastery)
			}
		})
	}
}

func TestBKTUpdateWithErrorTypeClampsParameters(t *testing.T) {
	// SYNTAX_ERROR: PSlip clamped to <= 0.5 even when input is high.
	highSlip := BKTState{PMastery: 0.5, PLearn: 0.3, PForget: 0.05, PSlip: 0.45, PGuess: 0.2}
	got := BKTUpdateWithErrorType(highSlip, false, "SYNTAX_ERROR")
	if got.PMastery < 0 || got.PMastery > 1 {
		t.Errorf("PMastery out of bounds: %f", got.PMastery)
	}
	// KNOWLEDGE_GAP: PGuess clamped to >= 0.05 even when input is low.
	lowGuess := BKTState{PMastery: 0.5, PLearn: 0.3, PForget: 0.05, PSlip: 0.1, PGuess: 0.06}
	got = BKTUpdateWithErrorType(lowGuess, false, "KNOWLEDGE_GAP")
	if got.PMastery < 0 || got.PMastery > 1 {
		t.Errorf("PMastery out of bounds: %f", got.PMastery)
	}
}
