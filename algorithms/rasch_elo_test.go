package algorithms

import (
	"math"
	"testing"
)

func TestRaschEloProbabilityMonotone(t *testing.T) {
	pLow := RaschEloProbability(-1, 0)
	pMid := RaschEloProbability(0, 0)
	pHigh := RaschEloProbability(1, 0)
	if !(pLow < pMid && pMid < pHigh) {
		t.Fatalf("probability should increase with ability: low=%f mid=%f high=%f", pLow, pMid, pHigh)
	}
	if !approxEqual(pMid, 0.5, 1e-12) {
		t.Fatalf("equal ability and difficulty should produce 0.5, got %f", pMid)
	}

	easier := RaschEloProbability(0, -1)
	harder := RaschEloProbability(0, 1)
	if !(harder < pMid && pMid < easier) {
		t.Fatalf("probability should decrease with difficulty: harder=%f mid=%f easier=%f", harder, pMid, easier)
	}
}

func TestRaschEloUpdateSuccessMovesAbilityAndDifficulty(t *testing.T) {
	state := NewRaschEloState(0, 0)
	next := RaschEloUpdate(state, true)

	if next.Ability <= state.Ability {
		t.Fatalf("success should increase ability: before=%f after=%f", state.Ability, next.Ability)
	}
	if next.Difficulty >= state.Difficulty {
		t.Fatalf("success should decrease difficulty: before=%f after=%f", state.Difficulty, next.Difficulty)
	}
	if next.Attempts != 1 || next.Successes != 1 || next.Failures != 0 {
		t.Fatalf("unexpected counts after success: %+v", next)
	}
}

func TestRaschEloUpdateFailureMovesAbilityAndDifficulty(t *testing.T) {
	state := NewRaschEloState(0, 0)
	next := RaschEloUpdate(state, false)

	if next.Ability >= state.Ability {
		t.Fatalf("failure should decrease ability: before=%f after=%f", state.Ability, next.Ability)
	}
	if next.Difficulty <= state.Difficulty {
		t.Fatalf("failure should increase difficulty: before=%f after=%f", state.Difficulty, next.Difficulty)
	}
	if next.Attempts != 1 || next.Successes != 0 || next.Failures != 1 {
		t.Fatalf("unexpected counts after failure: %+v", next)
	}
}

func TestRaschEloUpdateClamps(t *testing.T) {
	high := RaschEloUpdateWithK(RaschEloState{
		Ability:    math.Inf(1),
		Difficulty: math.Inf(-1),
		Attempts:   -10,
		Successes:  -5,
		Failures:   -2,
	}, true, math.Inf(1))
	if high.Ability != RaschEloMaxLogit || high.Difficulty != RaschEloMinLogit {
		t.Fatalf("expected high clamp to [%f,%f], got ability=%f difficulty=%f",
			RaschEloMaxLogit, RaschEloMinLogit, high.Ability, high.Difficulty)
	}
	if high.Attempts != 1 || high.Successes != 1 || high.Failures != 0 {
		t.Fatalf("negative counts should normalize before incrementing, got %+v", high)
	}

	low := RaschEloUpdateWithK(RaschEloState{
		Ability:    math.Inf(-1),
		Difficulty: math.Inf(1),
	}, false, math.Inf(1))
	if low.Ability != RaschEloMinLogit || low.Difficulty != RaschEloMaxLogit {
		t.Fatalf("expected low clamp to [%f,%f], got ability=%f difficulty=%f",
			RaschEloMinLogit, RaschEloMaxLogit, low.Ability, low.Difficulty)
	}

	neutral := NewRaschEloState(math.NaN(), math.NaN())
	if neutral.Ability != 0 || neutral.Difficulty != 0 {
		t.Fatalf("NaN logits should normalize to neutral state, got %+v", neutral)
	}
}

func TestRaschEloDeterminism(t *testing.T) {
	state := RaschEloState{
		Ability:    0.7,
		Difficulty: -0.2,
		Attempts:   4,
		Successes:  3,
		Failures:   1,
	}

	first := RaschEloUpdateWithK(state, false, 0.42)
	second := RaschEloUpdateWithK(state, false, 0.42)
	if first != second {
		t.Fatalf("same input should produce same output: first=%+v second=%+v", first, second)
	}
}
