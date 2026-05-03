// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"testing"
	"time"

	"tutor-mcp/models"
)

func TestBuildOLMSnapshot_PinnedConceptOverridesAutoFocus(t *testing.T) {
	store, raw := newOLMTestStore(t)
	seedLearner(t, raw, "L1")
	domainID := seedDomain(t, raw, "L1", "py",
		[]string{"vars", "loops", "funcs"},
		map[string][]string{"loops": {"vars"}, "funcs": {"loops"}},
		false)

	// Pin "funcs" — auto focus would have been "vars" (frontier).
	if err := store.SetPinnedConcept("L1", domainID, "funcs"); err != nil {
		t.Fatal(err)
	}
	snap, err := BuildOLMSnapshot(store, "L1", domainID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.FocusConcept != "funcs" {
		t.Fatalf("expected focus=funcs (pinned), got %q", snap.FocusConcept)
	}
}

func TestBuildOLMSnapshot_PinnedConceptDisappearedFromGraph_SilentClear(t *testing.T) {
	store, raw := newOLMTestStore(t)
	seedLearner(t, raw, "L1")
	domainID := seedDomain(t, raw, "L1", "py",
		[]string{"vars", "loops"},
		nil,
		false)
	if err := store.SetPinnedConcept("L1", domainID, "ghost"); err != nil {
		t.Fatal(err)
	}

	snap, err := BuildOLMSnapshot(store, "L1", domainID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snap.FocusConcept == "ghost" {
		t.Fatalf("ghost concept should not become focus")
	}
	got, err := store.GetDomainByID(domainID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PinnedConcept != "" {
		t.Fatalf("expected silent clear of stale pin, still %q", got.PinnedConcept)
	}
}

func TestBuildOLMSnapshot_NoPin_FallsBackToAuto(t *testing.T) {
	store, raw := newOLMTestStore(t)
	seedLearner(t, raw, "L1")
	domainID := seedDomain(t, raw, "L1", "py", []string{"vars"}, nil, false)
	snap, err := BuildOLMSnapshot(store, "L1", domainID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.FocusConcept != "vars" {
		t.Fatalf("expected auto focus=vars, got %q", snap.FocusConcept)
	}
}

func TestBuildOLMSnapshot_PinnedConceptPreservesAutoUrgency(t *testing.T) {
	// If the pinned concept ALSO has a higher-urgency auto signal (e.g.
	// forgetting alert → UrgencyCritical), the pin override must not downgrade
	// urgency to Info. The learner's "à reprendre vite" framing must survive.
	store, raw := newOLMTestStore(t)
	seedLearner(t, raw, "L1")
	domainID := seedDomain(t, raw, "L1", "py", []string{"loops"}, nil, false)

	// Seed "loops" in deep forgetting: stability=1.0, elapsed=30 days,
	// LastReview backdated 30 days — retrievability ≪ 0.30 → UrgencyCritical.
	cs := models.NewConceptState("L1", "loops")
	cs.PMastery = 0.40
	cs.Stability = 1.0
	cs.ElapsedDays = 30
	cs.Reps = 5
	cs.CardState = "review"
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	cs.LastReview = &old
	if err := store.UpsertConceptState(cs); err != nil {
		t.Fatal(err)
	}

	// Pin "loops" — same concept that the forgetting alert path would already pick.
	if err := store.SetPinnedConcept("L1", domainID, "loops"); err != nil {
		t.Fatal(err)
	}

	snap, err := BuildOLMSnapshot(store, "L1", domainID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.FocusConcept != "loops" {
		t.Fatalf("expected focus=loops, got %q", snap.FocusConcept)
	}
	if snap.FocusUrgency == models.UrgencyInfo {
		t.Fatalf("pin override downgraded urgency to Info; expected to preserve auto urgency (alert path)")
	}
}
