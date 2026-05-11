// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

import "math"

const (
	RaschEloMinLogit  = -6.0
	RaschEloMaxLogit  = 6.0
	RaschEloDefaultK  = 0.30
	RaschEloMaxK      = 1.0
	raschEloNeutral   = 0.0
	raschEloMinCount  = 0
	raschEloOutcomeOK = 1.0
)

// RaschEloState is the minimal auditable state for one learner/exercise pair.
// Ability and Difficulty share the same logit scale; counts are cumulative
// observations applied to this pair.
type RaschEloState struct {
	Ability    float64
	Difficulty float64
	Attempts   int
	Successes  int
	Failures   int
}

// NewRaschEloState returns a normalized Rasch/Elo state with zero counts.
func NewRaschEloState(ability, difficulty float64) RaschEloState {
	return RaschEloState{
		Ability:    raschEloClampLogit(ability),
		Difficulty: raschEloClampLogit(difficulty),
	}
}

// RaschEloProbability returns P(success) from ability and difficulty using a
// Rasch-style logistic curve.
func RaschEloProbability(ability, difficulty float64) float64 {
	return logistic(ability - difficulty)
}

// RaschEloSuccessProbability returns P(success) for the provided state.
func RaschEloSuccessProbability(state RaschEloState) float64 {
	return RaschEloProbability(state.Ability, state.Difficulty)
}

// RaschEloUpdate applies the default deterministic Elo correction.
func RaschEloUpdate(state RaschEloState, success bool) RaschEloState {
	return RaschEloUpdateWithK(state, success, RaschEloDefaultK)
}

// RaschEloUpdateWithK applies a symmetric Elo correction:
// ability += k*(outcome-p), difficulty -= k*(outcome-p).
//
// Non-finite logits are normalized to 0. Negative or non-finite k values are
// treated as 0; oversized k values are clamped to RaschEloMaxK.
func RaschEloUpdateWithK(state RaschEloState, success bool, k float64) RaschEloState {
	next := normalizeRaschEloState(state)
	p := RaschEloSuccessProbability(next)

	outcome := 0.0
	if success {
		outcome = raschEloOutcomeOK
	}

	delta := raschEloLearningRate(k) * (outcome - p)
	next.Ability = raschEloClampLogit(next.Ability + delta)
	next.Difficulty = raschEloClampLogit(next.Difficulty - delta)
	next.Attempts++
	if success {
		next.Successes++
	} else {
		next.Failures++
	}
	return next
}

func logistic(x float64) float64 {
	if math.IsNaN(x) {
		return 0.5
	}
	if x >= 0 {
		z := math.Exp(-x)
		return 1.0 / (1.0 + z)
	}
	z := math.Exp(x)
	return z / (1.0 + z)
}

func normalizeRaschEloState(state RaschEloState) RaschEloState {
	return RaschEloState{
		Ability:    raschEloClampLogit(state.Ability),
		Difficulty: raschEloClampLogit(state.Difficulty),
		Attempts:   maxInt(state.Attempts, raschEloMinCount),
		Successes:  maxInt(state.Successes, raschEloMinCount),
		Failures:   maxInt(state.Failures, raschEloMinCount),
	}
}

func raschEloLearningRate(k float64) float64 {
	if math.IsNaN(k) || k < 0 {
		return 0
	}
	if math.IsInf(k, 1) || k > RaschEloMaxK {
		return RaschEloMaxK
	}
	return k
}

func raschEloClampLogit(v float64) float64 {
	if math.IsNaN(v) {
		return raschEloNeutral
	}
	if math.IsInf(v, -1) {
		return RaschEloMinLogit
	}
	if math.IsInf(v, 1) {
		return RaschEloMaxLogit
	}
	return clamp(v, RaschEloMinLogit, RaschEloMaxLogit)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
