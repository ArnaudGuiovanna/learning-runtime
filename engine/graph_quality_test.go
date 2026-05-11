// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"testing"

	"tutor-mcp/models"
)

func TestEvaluateGraphQuality_OK(t *testing.T) {
	report := EvaluateGraphQuality(models.KnowledgeSpace{
		Concepts: []string{"variables", "functions", "closures", "goroutines"},
		Prerequisites: map[string][]string{
			"functions":  {"variables"},
			"closures":   {"functions"},
			"goroutines": {"functions"},
		},
	})
	if report.Quality != GraphQualityOK {
		t.Fatalf("quality = %s, want ok; issues=%+v", report.Quality, report.Issues)
	}
	if report.Metrics.ConceptCount != 4 || report.Metrics.EdgeCount != 3 {
		t.Fatalf("unexpected metrics: %+v", report.Metrics)
	}
	if report.ShouldAskLLMReview {
		t.Fatalf("did not expect LLM review prompt: %+v", report)
	}
}

func TestEvaluateGraphQuality_CriticalIssues(t *testing.T) {
	report := EvaluateGraphQuality(models.KnowledgeSpace{
		Concepts: []string{"a", "b", "a", "c"},
		Prerequisites: map[string][]string{
			"a":       {"b"},
			"b":       {"a"},
			"c":       {"ghost"},
			"unknown": {"a"},
		},
	})
	if report.Quality != GraphQualityCritical {
		t.Fatalf("quality = %s, want critical; issues=%+v", report.Quality, report.Issues)
	}
	for _, typ := range []string{"duplicate_concept", "unknown_prerequisite", "unknown_prerequisite_key", "cycle"} {
		if !graphQualityHasIssue(report, typ) {
			t.Fatalf("expected issue %q in %+v", typ, report.Issues)
		}
	}
	if !report.ShouldAskLLMReview || report.LLMRepairPrompt == "" {
		t.Fatalf("expected LLM repair prompt for critical report")
	}
}

func TestEvaluateGraphQuality_Warnings(t *testing.T) {
	report := EvaluateGraphQuality(models.KnowledgeSpace{
		Concepts:      []string{"Basics", "Go routines", "goroutines", "channels", "overview"},
		Prerequisites: map[string][]string{},
	})
	if report.Quality != GraphQualityWarning {
		t.Fatalf("quality = %s, want warning; issues=%+v", report.Quality, report.Issues)
	}
	for _, typ := range []string{"near_duplicate_concept", "vague_concept", "graph_too_flat", "isolated_concepts"} {
		if !graphQualityHasIssue(report, typ) {
			t.Fatalf("expected issue %q in %+v", typ, report.Issues)
		}
	}
	if !report.ShouldAskLLMReview || report.LLMRepairPrompt == "" {
		t.Fatalf("expected LLM repair prompt for warning report")
	}
}

func graphQualityHasIssue(report GraphQualityReport, typ string) bool {
	for _, issue := range report.Issues {
		if issue.Type == typ {
			return true
		}
	}
	return false
}
