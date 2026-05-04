// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

import "testing"

// TestThresholdIntegration_KSTGating uses an ambiguous mastery (0.75) that
// straddles the legacy and unified KST thresholds, then verifies that the
// frontier composition flips between profiles.
func TestThresholdIntegration_KSTGating(t *testing.T) {
	graph := KSTGraph{
		Concepts:      []string{"A", "B"},
		Prerequisites: map[string][]string{"B": {"A"}},
	}
	mastery := map[string]float64{"A": 0.75, "B": 0.0}

	t.Run("legacy: A=0.75 unlocks B (>=0.70)", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "off")
		f := ComputeFrontier(graph, mastery)
		if len(f) != 1 || f[0] != "B" {
			t.Errorf("legacy frontier: want [B], got %v", f)
		}
	})

	t.Run("unified: A=0.75 does NOT unlock B (<0.85)", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "on")
		f := ComputeFrontier(graph, mastery)
		// A is not yet "done" (0.75 < 0.85) and has no prereqs → frontier=[A].
		if len(f) != 1 || f[0] != "A" {
			t.Errorf("unified frontier: want [A], got %v", f)
		}
	})
}

// TestThresholdIntegration_BKTUnchanged verifies the BKT threshold is
// stable across profiles (always 0.85) so existing mastery-challenge gating
// behaviour is preserved.
func TestThresholdIntegration_BKTUnchanged(t *testing.T) {
	state := BKTState{PMastery: 0.86}
	t.Run("legacy", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "off")
		if !BKTIsMastered(state) {
			t.Error("0.86 must be mastered in legacy")
		}
	})
	t.Run("unified", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "on")
		if !BKTIsMastered(state) {
			t.Error("0.86 must be mastered in unified")
		}
	})
}

// TestThresholdIntegration_MidCollapse verifies that the intermediate
// threshold collapses to the BKT threshold under the unified profile,
// closing the 0.80–0.85 ambiguity zone where a concept was previously
// considered "mastered enough" for hint-independence but not for a
// mastery challenge.
func TestThresholdIntegration_MidCollapse(t *testing.T) {
	t.Run("legacy: PMastery=0.83 counts as mid-mastered (>=0.80)", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "off")
		if 0.83 < MasteryMid() {
			t.Errorf("legacy: 0.83 must be >= MasteryMid (%v)", MasteryMid())
		}
	})
	t.Run("unified: PMastery=0.83 does NOT count as mid-mastered (<0.85)", func(t *testing.T) {
		t.Setenv("REGULATION_THRESHOLD", "on")
		if 0.83 >= MasteryMid() {
			t.Errorf("unified: 0.83 must be < MasteryMid (%v)", MasteryMid())
		}
	})
}
