// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

import "math"

const (
	individualBKTMaxSlip  = 0.5
	individualBKTMaxGuess = 0.5
)

// IndividualBKTProfile summarizes learner/concept observations used to tune a
// single BKT update. All rates are expected in [0, 1]; invalid values are
// treated defensively so this layer remains pure and deterministic.
type IndividualBKTProfile struct {
	Observations        int
	SuccessRate         float64
	ErrorRate           float64
	AvgConfidence       float64
	HintsRate           float64
	OverconfidenceRate  float64
	Stability           float64
	AvgResponseTimeSecs float64
}

// IndividualBKTParameters reports the BKT parameters effectively used for one
// individualized update.
type IndividualBKTParameters struct {
	PLearn float64
	PSlip  float64
	PGuess float64
}

// IndividualBKTUpdateResult is the complete output of BKTUpdateIndividualized.
type IndividualBKTUpdateResult struct {
	State  BKTState
	Params IndividualBKTParameters
}

// BKTUpdateIndividualized applies a deterministic learner/concept adjustment
// before delegating to the standard BKT update. An empty profile keeps the
// existing BKT behaviour, including the current error-type heuristic.
func BKTUpdateIndividualized(state BKTState, profile IndividualBKTProfile, correct bool, errorType string) IndividualBKTUpdateResult {
	adjusted := sanitizeBKTProbabilities(state)
	weight := individualBKTEvidenceWeight(profile)

	if weight > 0 {
		signals := sanitizeIndividualBKTSignals(profile)
		strongStable := signals.success * (0.75 + 0.25*signals.confidence)
		overconfidence := math.Max(signals.overconfidence, clamp((signals.confidence-0.75)*4, 0, 1)*signals.error)

		learnDelta := weight * (0.06*strongStable + 0.04*signals.stability - 0.07*signals.error - 0.05*signals.hints - 0.06*overconfidence)
		slipDelta := weight * (-0.06*strongStable - 0.05*signals.stability + 0.06*signals.error + 0.04*signals.hints + 0.05*overconfidence)
		guessDelta := weight * (-0.04*strongStable - 0.02*signals.stability + 0.04*signals.error + 0.06*signals.hints + 0.05*overconfidence)

		adjusted.PLearn = finiteClamp(adjusted.PLearn+learnDelta, 0, 1)
		adjusted.PSlip = finiteClamp(adjusted.PSlip+slipDelta, 0, individualBKTMaxSlip)
		adjusted.PGuess = finiteClamp(adjusted.PGuess+guessDelta, 0, individualBKTMaxGuess)
	}

	if !correct {
		switch errorType {
		case "SYNTAX_ERROR":
			adjusted.PSlip = finiteClamp(adjusted.PSlip+0.15, 0, individualBKTMaxSlip)
		case "KNOWLEDGE_GAP":
			adjusted.PGuess = finiteClamp(adjusted.PGuess-0.10, 0.05, individualBKTMaxGuess)
			adjusted.PLearn = finiteClamp(adjusted.PLearn-0.03*weight, 0, 1)
		case "LOGIC_ERROR":
			adjusted.PSlip = finiteClamp(adjusted.PSlip+0.03*weight, 0, individualBKTMaxSlip)
		}
	}

	next := BKTUpdate(adjusted, correct)
	next = sanitizeBKTProbabilities(next)

	return IndividualBKTUpdateResult{
		State: next,
		Params: IndividualBKTParameters{
			PLearn: adjusted.PLearn,
			PSlip:  adjusted.PSlip,
			PGuess: adjusted.PGuess,
		},
	}
}

type individualBKTSignals struct {
	success        float64
	error          float64
	confidence     float64
	hints          float64
	overconfidence float64
	stability      float64
}

func sanitizeIndividualBKTSignals(profile IndividualBKTProfile) individualBKTSignals {
	success := finiteClamp(profile.SuccessRate, 0, 1)
	errorRate := finiteClamp(profile.ErrorRate, 0, 1)
	if profile.Observations > 0 && profile.ErrorRate == 0 {
		errorRate = 1 - success
	}

	confidence := finiteClamp(profile.AvgConfidence, 0, 1)
	if !isFinite(profile.AvgConfidence) || profile.AvgConfidence == 0 {
		confidence = 0.5
	}

	return individualBKTSignals{
		success:        success,
		error:          errorRate,
		confidence:     confidence,
		hints:          finiteClamp(profile.HintsRate, 0, 1),
		overconfidence: finiteClamp(profile.OverconfidenceRate, 0, 1),
		stability:      finiteClamp(profile.Stability, 0, 1),
	}
}

func individualBKTEvidenceWeight(profile IndividualBKTProfile) float64 {
	if profile.Observations > 0 {
		return finiteClamp(float64(profile.Observations)/20, 0, 1)
	}
	if profile.SuccessRate != 0 ||
		profile.ErrorRate != 0 ||
		profile.AvgConfidence != 0 ||
		profile.HintsRate != 0 ||
		profile.OverconfidenceRate != 0 ||
		profile.Stability != 0 ||
		profile.AvgResponseTimeSecs != 0 {
		return 0.5
	}
	return 0
}

func sanitizeBKTProbabilities(state BKTState) BKTState {
	return BKTState{
		PMastery: finiteClamp(state.PMastery, 0, 1),
		PLearn:   finiteClamp(state.PLearn, 0, 1),
		PForget:  finiteClamp(state.PForget, 0, 1),
		PSlip:    finiteClamp(state.PSlip, 0, 1),
		PGuess:   finiteClamp(state.PGuess, 0, 1),
	}
}

func finiteClamp(v, min, max float64) float64 {
	if !isFinite(v) {
		return min
	}
	return clamp(v, min, max)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
