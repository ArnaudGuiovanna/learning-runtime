// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package db

import (
	"testing"

	"tutor-mcp/models"
)

func TestSetPinnedConcept_HappyPath(t *testing.T) {
	store := setupTestDB(t)
	d, err := store.CreateDomainWithValueFramings("L1", "py", "", models.KnowledgeSpace{
		Concepts:      []string{"a", "b"},
		Prerequisites: map[string][]string{"b": {"a"}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPinnedConcept("L1", d.ID, "b"); err != nil {
		t.Fatalf("set pin: %v", err)
	}
	got, err := store.GetDomainByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PinnedConcept != "b" {
		t.Fatalf("expected pin=b, got %q", got.PinnedConcept)
	}
}

func TestSetPinnedConcept_Clear(t *testing.T) {
	store := setupTestDB(t)
	d, _ := store.CreateDomainWithValueFramings("L1", "py", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	}, "")
	_ = store.SetPinnedConcept("L1", d.ID, "a")
	if err := store.SetPinnedConcept("L1", d.ID, ""); err != nil {
		t.Fatalf("clear pin: %v", err)
	}
	got, _ := store.GetDomainByID(d.ID)
	if got.PinnedConcept != "" {
		t.Fatalf("expected empty pin after clear, got %q", got.PinnedConcept)
	}
}

func TestSetPinnedConcept_RejectMismatchedOwnership(t *testing.T) {
	store := setupTestDB(t)
	d, _ := store.CreateDomainWithValueFramings("L1", "py", "", models.KnowledgeSpace{
		Concepts: []string{"a"},
	}, "")
	// Use a learner ID that doesn't exist (or matches a different learner).
	// The point is the WHERE clause has learner_id mismatch → 0 rows affected.
	err := store.SetPinnedConcept("L_someone_else", d.ID, "a")
	if err == nil {
		t.Fatalf("expected error for mismatched ownership")
	}
}
