// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package algorithms

import (
	"os"
	"testing"
)

// TestMain anchors the algorithms package's test suite to the legacy
// threshold profile by setting REGULATION_THRESHOLD=off. This preserves
// the existing fixtures (e.g., TestComputeFrontier with mastery=0.75
// expecting the unlock at KST=0.70) without mutating their values.
//
// Tests that explicitly need the unified profile call
// t.Setenv("REGULATION_THRESHOLD", "on") (or any non-"off" value),
// which overrides this for the duration of the test only.
//
// Note: production default has been promoted to "unified" — only "off"
// opts back to legacy. The test suite locks to legacy by design here so
// that legacy fixtures keep documenting the legacy semantic; new tests
// in *_unified_test.go assert the unified semantic explicitly. See
// docs/regulation-design/07-threshold-resolver.md §6.5.
func TestMain(m *testing.M) {
	_ = os.Setenv("REGULATION_THRESHOLD", "off")
	os.Exit(m.Run())
}
