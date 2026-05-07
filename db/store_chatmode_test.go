// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package db

import "testing"

func TestChatMode_DefaultFalse(t *testing.T) {
	store := setupTestDB(t)

	enabled, err := store.GetChatModeEnabled("L1")
	if err != nil {
		t.Fatalf("GetChatModeEnabled: %v", err)
	}
	if enabled {
		t.Fatalf("default should be false, got true")
	}
}

func TestChatMode_RoundTrip(t *testing.T) {
	store := setupTestDB(t)

	if err := store.SetChatModeEnabled("L1", true); err != nil {
		t.Fatalf("SetChatModeEnabled true: %v", err)
	}
	enabled, err := store.GetChatModeEnabled("L1")
	if err != nil {
		t.Fatalf("GetChatModeEnabled after true: %v", err)
	}
	if !enabled {
		t.Fatalf("expected true after set, got false")
	}

	if err := store.SetChatModeEnabled("L1", false); err != nil {
		t.Fatalf("SetChatModeEnabled false: %v", err)
	}
	enabled, _ = store.GetChatModeEnabled("L1")
	if enabled {
		t.Fatalf("expected false after re-set, got true")
	}
}

func TestChatMode_UnknownLearner_NoError_DefaultFalse(t *testing.T) {
	store := setupTestDB(t)
	enabled, err := store.GetChatModeEnabled("NEVER_SEEN")
	if err != nil {
		t.Fatalf("expected nil err for unknown learner, got %v", err)
	}
	if enabled {
		t.Fatalf("expected false for unknown learner, got true")
	}
}
