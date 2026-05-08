// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import "testing"

// TestDefaultRecentInteractionsWindow_Value pins the documented value.
// Changing it should be a deliberate decision (touch this test together
// with the doc comment in window.go).
func TestDefaultRecentInteractionsWindow_Value(t *testing.T) {
	if DefaultRecentInteractionsWindow != 20 {
		t.Fatalf("DefaultRecentInteractionsWindow = %d; want 20", DefaultRecentInteractionsWindow)
	}
}

// TestDefaultRecentInteractionsWindow_Sane guards against accidental
// regression to a pathological window (0, negative, or so large that
// it dwarfs the alert pipeline). The constant must stay strictly
// positive and within a sensible range for the alert / OLM consumers.
func TestDefaultRecentInteractionsWindow_Sane(t *testing.T) {
	if DefaultRecentInteractionsWindow <= 0 {
		t.Fatalf("DefaultRecentInteractionsWindow must be > 0, got %d", DefaultRecentInteractionsWindow)
	}
	if DefaultRecentInteractionsWindow > 1000 {
		t.Fatalf("DefaultRecentInteractionsWindow unreasonably large: %d", DefaultRecentInteractionsWindow)
	}
}
