// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package memory

import (
	"strings"
	"testing"
	"time"
)

func TestWriteReadAndReplaceSection(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ROOT", t.TempDir())
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "true")

	if err := Write(WriteRequest{
		LearnerID:  "L1",
		Scope:      ScopeMemory,
		Operation:  OpReplaceSection,
		SectionKey: "Current state",
		Content:    "First version.",
	}); err != nil {
		t.Fatalf("Write replace section: %v", err)
	}
	if err := Write(WriteRequest{
		LearnerID:  "L1",
		Scope:      ScopeMemory,
		Operation:  OpReplaceSection,
		SectionKey: "Current state",
		Content:    "Second version.",
	}); err != nil {
		t.Fatalf("Write replace section again: %v", err)
	}
	got, err := Read("L1", ScopeMemory, "")
	if err != nil {
		t.Fatalf("Read memory: %v", err)
	}
	if strings.Count(got, "## Current state") != 1 {
		t.Fatalf("section duplicated:\n%s", got)
	}
	if !strings.Contains(got, "Second version.") || strings.Contains(got, "First version.") {
		t.Fatalf("section was not replaced:\n%s", got)
	}
}

func TestWriteSessionUsesTimestampFilenameAndParsesYAML(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ROOT", t.TempDir())
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "true")

	ts := time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC)
	content := `---
timestamp: 2026-05-14T09:30:00Z
duration_minutes: 47
affect_start: focused
affect_end: satisfied
energy_level: high
concepts_touched: ["probabilites_conditionnelles", "bayes_theorem"]
session_type: deep_dive
novelty_flag: true
---

## Summary
The learner connected Bayes and conditional probabilities.`
	if err := Write(WriteRequest{
		LearnerID: "L1",
		Scope:     ScopeSession,
		Timestamp: ts,
		Operation: OpReplaceFile,
		Content:   content,
	}); err != nil {
		t.Fatalf("Write session: %v", err)
	}
	sessions, err := ListSessions("L1")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || !sessions[0].Equal(ts) {
		t.Fatalf("sessions = %v, want [%v]", sessions, ts)
	}
	raw, err := Read("L1", ScopeSession, ts.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("Read session: %v", err)
	}
	payload, err := ParseSessionPayload(ts, raw)
	if err != nil {
		t.Fatalf("ParseSessionPayload: %v", err)
	}
	if payload.Frontmatter["affect_start"] != "focused" {
		t.Fatalf("frontmatter not parsed: %#v", payload.Frontmatter)
	}
	if !truthy(payload.Frontmatter["novelty_flag"]) {
		t.Fatalf("novelty_flag not parsed as truthy: %#v", payload.Frontmatter["novelty_flag"])
	}
	if !strings.Contains(payload.Body, "Bayes") {
		t.Fatalf("body not parsed: %q", payload.Body)
	}
}

func TestConceptSlugMayContainSlash(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ROOT", t.TempDir())
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "true")

	if err := Write(WriteRequest{
		LearnerID:   "L1",
		Scope:       ScopeConcept,
		ConceptSlug: "encoding/json",
		Operation:   OpReplaceSection,
		SectionKey:  "Current state",
		Content:     "Needs more transfer practice.",
	}); err != nil {
		t.Fatalf("Write concept: %v", err)
	}
	got, err := Read("L1", ScopeConcept, "encoding/json")
	if err != nil {
		t.Fatalf("Read concept: %v", err)
	}
	if !strings.Contains(got, "Needs more transfer practice.") {
		t.Fatalf("unexpected concept notes: %q", got)
	}
	concepts, err := ListConcepts("L1")
	if err != nil {
		t.Fatalf("ListConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0] != "encoding/json" {
		t.Fatalf("concepts = %v, want encoding/json", concepts)
	}
}

func TestEnabledDefaultsOnAndCanBeDisabled(t *testing.T) {
	t.Setenv("TUTOR_MCP_MEMORY_ENABLED", "")
	if !Enabled() {
		t.Fatal("memory should be enabled by default")
	}
	for _, value := range []string{"0", "false", "off", "no"} {
		t.Setenv("TUTOR_MCP_MEMORY_ENABLED", value)
		if Enabled() {
			t.Fatalf("memory should be disabled for %q", value)
		}
	}
}
