// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

type BKTState struct {
	PMastery float64
	PLearn   float64
	PForget  float64
	PSlip    float64
	PGuess   float64
}

// bktEpsilon is the minimum allowed marginal probability used to clamp the
// Bayesian denominator in BKTUpdate. With degenerate inputs (e.g.
// PMastery=0 and PGuess=0 on a "correct" observation), pCorrect collapses to
// zero and the 0/0 division yields NaN, poisoning every downstream consumer.
// Clamping to a small positive number preserves the standard update path for
// sane inputs and short-circuits the NaN without behaviour change.
const bktEpsilon = 1e-9

// Mastery thresholds are exposed via accessors in thresholds.go (MasteryBKT,
// MasteryKST, MasteryMid). The bascule REGULATION_THRESHOLD=on collapses
// them to a single 0.85 — see docs/regulation-design/07-threshold-resolver.md.

func BKTUpdate(state BKTState, correct bool) BKTState {
	var pMasteryGivenObs float64
	if correct {
		pCorrectMastery := 1.0 - state.PSlip
		pCorrectNotMastery := state.PGuess
		pCorrect := pCorrectMastery*state.PMastery + pCorrectNotMastery*(1-state.PMastery)
		// Guard against pCorrect==0 (e.g. PMastery=0 ∧ PGuess=0) which would
		// produce a NaN. Clamping to bktEpsilon makes the posterior fall back
		// to ~0, which is the correct limit when the observation has zero
		// modelled probability under either hypothesis.
		if pCorrect < bktEpsilon {
			pCorrect = bktEpsilon
		}
		pMasteryGivenObs = pCorrectMastery * state.PMastery / pCorrect
	} else {
		pIncorrectMastery := state.PSlip
		pIncorrectNotMastery := 1.0 - state.PGuess
		pIncorrect := pIncorrectMastery*state.PMastery + pIncorrectNotMastery*(1-state.PMastery)
		// Same guard as above for the incorrect branch (e.g. PMastery=0 ∧
		// PGuess=1, or PMastery=1 ∧ PSlip=0).
		if pIncorrect < bktEpsilon {
			pIncorrect = bktEpsilon
		}
		pMasteryGivenObs = pIncorrectMastery * state.PMastery / pIncorrect
	}
	newPMastery := pMasteryGivenObs*(1-state.PForget) + (1-pMasteryGivenObs)*state.PLearn
	result := state
	result.PMastery = clamp(newPMastery, 0, 1)
	return result
}

// BKTUpdateWithErrorType adjusts BKT parameters based on error type before updating.
// SYNTAX_ERROR: careless mistake — higher slip, less penalizing to mastery.
// KNOWLEDGE_GAP: genuine lack of understanding — lower guess, more penalizing.
// LOGIC_ERROR or empty: standard BKT update.
func BKTUpdateWithErrorType(state BKTState, correct bool, errorType string) BKTState {
	if correct || errorType == "" {
		return BKTUpdate(state, correct)
	}

	adjusted := state
	switch errorType {
	case "SYNTAX_ERROR":
		// Syntax errors indicate carelessness, not lack of knowledge.
		// Temporarily boost PSlip to reduce mastery penalty.
		adjusted.PSlip = clamp(state.PSlip+0.15, 0, 0.5)
	case "KNOWLEDGE_GAP":
		// Genuine knowledge gap — reduce PGuess to penalize more.
		adjusted.PGuess = clamp(state.PGuess-0.10, 0.05, 0.5)
	}
	// LOGIC_ERROR uses standard parameters

	return BKTUpdate(adjusted, correct)
}

func BKTIsMastered(state BKTState) bool {
	return state.PMastery >= MasteryBKT()
}
