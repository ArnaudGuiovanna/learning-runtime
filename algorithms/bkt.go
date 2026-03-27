package algorithms

type BKTState struct {
	PMastery float64
	PLearn   float64
	PForget  float64
	PSlip    float64
	PGuess   float64
}

const BKTMasteryThreshold = 0.85

func BKTUpdate(state BKTState, correct bool) BKTState {
	var pMasteryGivenObs float64
	if correct {
		pCorrectMastery := 1.0 - state.PSlip
		pCorrectNotMastery := state.PGuess
		pCorrect := pCorrectMastery*state.PMastery + pCorrectNotMastery*(1-state.PMastery)
		pMasteryGivenObs = pCorrectMastery * state.PMastery / pCorrect
	} else {
		pIncorrectMastery := state.PSlip
		pIncorrectNotMastery := 1.0 - state.PGuess
		pIncorrect := pIncorrectMastery*state.PMastery + pIncorrectNotMastery*(1-state.PMastery)
		pMasteryGivenObs = pIncorrectMastery * state.PMastery / pIncorrect
	}
	newPMastery := pMasteryGivenObs*(1-state.PForget) + (1-pMasteryGivenObs)*state.PLearn
	result := state
	result.PMastery = clamp(newPMastery, 0, 1)
	return result
}

func BKTIsMastered(state BKTState) bool {
	return state.PMastery >= BKTMasteryThreshold
}
