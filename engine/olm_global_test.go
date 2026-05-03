// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"testing"
	"time"
)

func TestGlobalOLMSnapshot_TypesCompile(t *testing.T) {
	g := &GlobalOLMSnapshot{
		Streak:     3,
		TotalSolid: 12,
		Domains: []DomainSummary{
			{DomainID: "d1", DomainName: "math", Solid: 5, KSTProgress: 0.6},
		},
		CalibrationHistory:  []TimePoint{{Day: "2026-05-03", Value: -1.2}},
		AutonomyHistory:     []TimePoint{{Day: "2026-05-03", Value: 0.7}},
		SatisfactionHistory: []TimePoint{{Day: "2026-05-03", Value: 3.0}},
		Goals: []GoalProgress{
			{DomainID: "d1", PersonalGoal: "g", Progress: 0.6},
		},
		RecentEvents: []LearnerEvent{
			{At: time.Now().UTC(), Kind: "mastery_threshold", Concept: "x", Message: "x atteint le seuil"},
		},
	}
	if g.TotalSolid != 12 || len(g.Domains) != 1 || len(g.Goals) != 1 {
		t.Errorf("unexpected shape: %+v", g)
	}
}
