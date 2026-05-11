// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"math"
	"strings"
	"testing"
)

func TestNormalizeRubricJSONStableObject(t *testing.T) {
	got, warnings, err := normalizeRubricJSON(`{
		"criteria": [
			{"id": "correctness", "description": "Right answer", "max_score": 2},
			{"id": "reasoning", "max_score": 1}
		],
		"passing_score": 2.5
	}`)
	if err != nil {
		t.Fatalf("normalizeRubricJSON returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	criteria := rubricCriteriaForTest(t, got)
	if len(criteria) != 2 {
		t.Fatalf("criteria length = %d, want 2", len(criteria))
	}
	if criteria[0]["id"] != "correctness" {
		t.Fatalf("criteria[0].id = %v", criteria[0]["id"])
	}
	if criteria[0]["description"] != "Right answer" {
		t.Fatalf("criteria[0].description = %v", criteria[0]["description"])
	}
	assertRubricFloat(t, criteria[0]["max_score"], 2)
	assertRubricFloat(t, got["passing_score"], 2.5)
}

func TestNormalizeRubricJSONLegacyShapes(t *testing.T) {
	got, warnings, err := normalizeRubricJSON(`{
		"scale": "0-1",
		"criteria": {
			"Correctness": {"description": "answer is right", "weight": 0.7},
			"Clear explanation": "clear enough"
		}
	}`)
	if err != nil {
		t.Fatalf("normalizeRubricJSON returned error: %v", err)
	}
	requireRubricWarning(t, warnings, "criteria object normalized")
	requireRubricWarning(t, warnings, "weight normalized as max_score")
	requireRubricWarning(t, warnings, "string normalized with max_score=1")

	criteria := rubricCriteriaForTest(t, got)
	if len(criteria) != 2 {
		t.Fatalf("criteria length = %d, want 2", len(criteria))
	}
	if criteria[0]["id"] != "clear_explanation" {
		t.Fatalf("criteria[0].id = %v", criteria[0]["id"])
	}
	if criteria[0]["description"] != "clear enough" {
		t.Fatalf("criteria[0].description = %v", criteria[0]["description"])
	}
	assertRubricFloat(t, criteria[0]["max_score"], 1)
	if criteria[1]["id"] != "correctness" {
		t.Fatalf("criteria[1].id = %v", criteria[1]["id"])
	}
	assertRubricFloat(t, criteria[1]["max_score"], 0.7)
}

func TestNormalizeRubricScoreJSONStableWithRubric(t *testing.T) {
	rubric, _, err := normalizeRubricJSON(`{
		"criteria": [
			{"id": "correctness", "max_score": 1},
			{"id": "reasoning", "max_score": 2}
		]
	}`)
	if err != nil {
		t.Fatalf("normalizeRubricJSON returned error: %v", err)
	}

	got, warnings, err := normalizeRubricScoreJSON(`{
		"criteria_scores": [
			{"id": "correctness", "score": 1, "evidence": "exact"},
			{"id": "reasoning", "score": 1.5, "error_type": "LOGIC_ERROR"}
		],
		"summary": "solid",
		"confidence": 0.75
	}`, rubric)
	if err != nil {
		t.Fatalf("normalizeRubricScoreJSON returned error: %v", err)
	}
	requireRubricWarning(t, warnings, "max_total missing; inferred from rubric_json")

	scores := rubricScoresForTest(t, got)
	if len(scores) != 2 {
		t.Fatalf("criteria_scores length = %d, want 2", len(scores))
	}
	if scores[0]["id"] != "correctness" || scores[0]["evidence"] != "exact" {
		t.Fatalf("unexpected first score: %#v", scores[0])
	}
	if scores[1]["id"] != "reasoning" || scores[1]["error_type"] != "LOGIC_ERROR" {
		t.Fatalf("unexpected second score: %#v", scores[1])
	}
	assertRubricFloat(t, got["total"], 2.5)
	assertRubricFloat(t, got["max_total"], 3)
	assertRubricFloat(t, got["confidence"], 0.75)
	if got["summary"] != "solid" {
		t.Fatalf("summary = %v", got["summary"])
	}
}

func TestNormalizeRubricScoreJSONLegacyMap(t *testing.T) {
	rubric, _, err := normalizeRubricJSON(`{
		"criteria": [
			{"id": "Correctness", "max_score": 1},
			{"id": "Reasoning", "max_score": 2}
		]
	}`)
	if err != nil {
		t.Fatalf("normalizeRubricJSON returned error: %v", err)
	}

	got, warnings, err := normalizeRubricScoreJSON(`{
		"overall": 2.5,
		"criteria_scores": {
			"Correctness": {"score": "1", "evidence": "ok"},
			"Reasoning": 1.5
		},
		"feedback": "good",
		"confidence": "0.8"
	}`, rubric)
	if err != nil {
		t.Fatalf("normalizeRubricScoreJSON returned error: %v", err)
	}
	requireRubricWarning(t, warnings, "criteria_scores object normalized")
	requireRubricWarning(t, warnings, "score string coerced")
	requireRubricWarning(t, warnings, "confidence string coerced")

	scores := rubricScoresForTest(t, got)
	if scores[0]["id"] != "correctness" {
		t.Fatalf("scores[0].id = %v", scores[0]["id"])
	}
	assertRubricFloat(t, scores[0]["score"], 1)
	if scores[1]["id"] != "reasoning" {
		t.Fatalf("scores[1].id = %v", scores[1]["id"])
	}
	assertRubricFloat(t, got["total"], 2.5)
	assertRubricFloat(t, got["max_total"], 3)
	if got["summary"] != "good" {
		t.Fatalf("summary = %v", got["summary"])
	}
}

func TestNormalizeRubricScoreJSONScalar(t *testing.T) {
	got, warnings, err := normalizeRubricScoreJSON(`0.8`, nil)
	if err != nil {
		t.Fatalf("normalizeRubricScoreJSON returned error: %v", err)
	}
	requireRubricWarning(t, warnings, "scalar normalized")

	scores := rubricScoresForTest(t, got)
	if len(scores) != 1 || scores[0]["id"] != "overall" {
		t.Fatalf("unexpected scores: %#v", scores)
	}
	assertRubricFloat(t, scores[0]["score"], 0.8)
	assertRubricFloat(t, got["total"], 0.8)
	assertRubricFloat(t, got["max_total"], 1)
}

func TestNormalizeRubricScoreJSONCrossSchemaWarnings(t *testing.T) {
	rubric, _, err := normalizeRubricJSON(`{
		"criteria": [
			{"id": "correctness", "max_score": 1},
			{"id": "reasoning", "max_score": 1}
		]
	}`)
	if err != nil {
		t.Fatalf("normalizeRubricJSON returned error: %v", err)
	}

	got, warnings, err := normalizeRubricScoreJSON(`{
		"criteria_scores": {
			"correctness": 1.2,
			"extra": 0.5
		}
	}`, rubric)
	if err != nil {
		t.Fatalf("normalizeRubricScoreJSON returned error: %v", err)
	}
	requireRubricWarning(t, warnings, "score exceeds rubric_json max_score")
	requireRubricWarning(t, warnings, "extra")
	requireRubricWarning(t, warnings, "missing rubric criterion")
	assertRubricFloat(t, got["total"], 1.7)
	assertRubricFloat(t, got["max_total"], 2)
}

func TestNormalizeRubricSchemaRejectsInvalidInputs(t *testing.T) {
	if _, _, err := normalizeRubricJSON(`42`); err == nil {
		t.Fatal("expected scalar rubric_json to be rejected")
	}
	if _, _, err := normalizeRubricJSON(`{"criteria":[{"id":"x","max_score":0}]}`); err == nil {
		t.Fatal("expected zero max_score to be rejected")
	}
	if _, _, err := normalizeRubricScoreJSON(`{"criteria_scores":[{"id":"x","evidence":"missing score"}]}`, nil); err == nil {
		t.Fatal("expected missing score to be rejected")
	}
	if _, _, err := normalizeRubricScoreJSON(`{"criteria_scores":{"x":1},"confidence":2}`, nil); err == nil {
		t.Fatal("expected out-of-range confidence to be rejected")
	}
}

func rubricCriteriaForTest(t *testing.T, got map[string]any) []map[string]any {
	t.Helper()
	criteria, ok := got["criteria"].([]map[string]any)
	if !ok {
		t.Fatalf("criteria type = %T, want []map[string]any", got["criteria"])
	}
	return criteria
}

func rubricScoresForTest(t *testing.T, got map[string]any) []map[string]any {
	t.Helper()
	scores, ok := got["criteria_scores"].([]map[string]any)
	if !ok {
		t.Fatalf("criteria_scores type = %T, want []map[string]any", got["criteria_scores"])
	}
	return scores
}

func assertRubricFloat(t *testing.T, got any, want float64) {
	t.Helper()
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("value type = %T, want float64", got)
	}
	if math.Abs(f-want) > 1e-9 {
		t.Fatalf("value = %v, want %v", f, want)
	}
}

func requireRubricWarning(t *testing.T, warnings []string, needle string) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return
		}
	}
	t.Fatalf("expected warning containing %q, got %v", needle, warnings)
}
