// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

import "testing"

func TestMasteryThresholds_LegacyOptOut(t *testing.T) {
	// Default in production is now unified; "off" opts out to legacy.
	// Explicit setenv in case TestMain is bypassed by a different harness.
	t.Setenv("REGULATION_THRESHOLD", "off")
	if got := MasteryBKT(); got != 0.85 {
		t.Errorf("MasteryBKT legacy: want 0.85, got %v", got)
	}
	if got := MasteryKST(); got != 0.70 {
		t.Errorf("MasteryKST legacy: want 0.70, got %v", got)
	}
	if got := MasteryMid(); got != 0.80 {
		t.Errorf("MasteryMid legacy: want 0.80, got %v", got)
	}
}

func TestMasteryThresholds_Unified(t *testing.T) {
	t.Setenv("REGULATION_THRESHOLD", "on")
	if got := MasteryBKT(); got != 0.85 {
		t.Errorf("MasteryBKT unified: want 0.85, got %v", got)
	}
	if got := MasteryKST(); got != 0.85 {
		t.Errorf("MasteryKST unified: want 0.85, got %v", got)
	}
	if got := MasteryMid(); got != 0.85 {
		t.Errorf("MasteryMid unified: want 0.85, got %v", got)
	}
}

// TestMasteryThresholds_OptOutStrictness verifies that only the exact
// literal "off" opts out to legacy. Common typos ("Off", "OFF", "false",
// "0", " off") fall through to the unified default — operators must type
// the canonical opt-out exactly, which protects against accidental
// rollbacks during a panic deploy.
func TestMasteryThresholds_OptOutStrictness(t *testing.T) {
	cases := []struct {
		flag    string
		wantKST float64 // 0.85 if unified (default), 0.70 if legacy
	}{
		{"off", 0.70},      // canonical opt-out
		{"Off", 0.85},      // typo → unified
		{"OFF", 0.85},      // typo → unified
		{" off", 0.85},     // typo → unified
		{"off ", 0.85},     // typo → unified
		{"false", 0.85},    // not "off" → unified
		{"0", 0.85},        // not "off" → unified
		{"disabled", 0.85}, // not "off" → unified
		{"", 0.85},         // unset-equivalent → unified (the new default)
		{"on", 0.85},       // explicit unified
	}
	for _, c := range cases {
		t.Run("flag="+c.flag, func(t *testing.T) {
			t.Setenv("REGULATION_THRESHOLD", c.flag)
			if got := MasteryKST(); got != c.wantKST {
				t.Errorf("flag %q: MasteryKST want %v, got %v", c.flag, c.wantKST, got)
			}
		})
	}
}

func TestRetentionThresholds_NamedForgettingBands(t *testing.T) {
	if got := RetentionAlertWarningThreshold; got != 0.40 {
		t.Errorf("RetentionAlertWarningThreshold: want 0.40, got %v", got)
	}
	if got := RetentionAlertCriticalThreshold; got != 0.30 {
		t.Errorf("RetentionAlertCriticalThreshold: want 0.30, got %v", got)
	}
	if got := RetentionRecallRoutingThreshold; got != 0.50 {
		t.Errorf("RetentionRecallRoutingThreshold: want 0.50, got %v", got)
	}
	if !(RetentionAlertCriticalThreshold < RetentionAlertWarningThreshold &&
		RetentionAlertWarningThreshold < RetentionRecallRoutingThreshold) {
		t.Fatalf(
			"retention thresholds must be ordered critical < warning < routing, got critical=%v warning=%v routing=%v",
			RetentionAlertCriticalThreshold,
			RetentionAlertWarningThreshold,
			RetentionRecallRoutingThreshold,
		)
	}
}
