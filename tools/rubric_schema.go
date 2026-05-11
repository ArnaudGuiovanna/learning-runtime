// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// normalizeRubricJSON parses rubric_json and returns a canonical map shaped as:
// {"criteria":[{"id","description?","max_score"}], "passing_score?"}.
func normalizeRubricJSON(raw string) (map[string]any, []string, error) {
	parsed, err := parseRubricSchemaJSON("rubric_json", raw)
	if err != nil || parsed == nil {
		return nil, nil, err
	}
	return normalizeRubricValue(parsed)
}

// normalizeRubricScoreJSON parses rubric_score_json and returns a canonical map
// shaped as:
// {"criteria_scores":[{"id","score","evidence?","error_type?"}],
//
//	"total", "max_total", "summary?", "confidence?"}.
//
// normalizedRubric is optional. When provided, its criteria ids and max_score
// values are used to infer max_total and emit cross-schema warnings.
func normalizeRubricScoreJSON(raw string, normalizedRubric map[string]any) (map[string]any, []string, error) {
	parsed, err := parseRubricSchemaJSON("rubric_score_json", raw)
	if err != nil || parsed == nil {
		return nil, nil, err
	}
	return normalizeRubricScoreValue(parsed, normalizedRubric)
}

// normalizeRubricValue normalizes an already-decoded rubric value. It is kept
// separate from normalizeRubricJSON so callers that already decoded JSON with
// json.Decoder.UseNumber can avoid a second parse.
func normalizeRubricValue(value any) (map[string]any, []string, error) {
	var warnings []string
	var criteriaValue any
	var passingValue any
	var hasPassing bool

	switch v := value.(type) {
	case map[string]any:
		if c, _, ok := lookupRubricField(v, "criteria"); ok {
			criteriaValue = c
		} else {
			legacy := legacyRubricCriteriaMap(v)
			if len(legacy) == 0 {
				return nil, warnings, fmt.Errorf("rubric_json must contain criteria")
			}
			criteriaValue = legacy
			warnings = append(warnings, "rubric_json has no criteria field; normalized non-metadata object keys as criteria")
		}
		passingValue, _, hasPassing = lookupRubricField(v, "passing_score", "passingScore", "pass_score", "passScore", "threshold")
	case []any:
		criteriaValue = v
		warnings = append(warnings, "rubric_json top-level array normalized as criteria")
	default:
		return nil, warnings, fmt.Errorf("rubric_json must be a JSON object or array of criteria")
	}

	criteria, maxTotal, err := normalizeRubricCriteria(criteriaValue, &warnings)
	if err != nil {
		return nil, warnings, err
	}
	if len(criteria) == 0 {
		return nil, warnings, fmt.Errorf("rubric_json.criteria must contain at least one criterion")
	}

	out := map[string]any{"criteria": criteria}
	if hasPassing {
		passing, coerced, ok := rubricFiniteNumber(passingValue)
		if !ok || passing < 0 {
			return nil, warnings, fmt.Errorf("rubric_json.passing_score must be a non-negative finite number")
		}
		if coerced {
			warnings = append(warnings, "rubric_json.passing_score string coerced to number")
		}
		if passing > maxTotal {
			warnings = append(warnings, "rubric_json.passing_score exceeds criteria max_total")
		}
		out["passing_score"] = passing
	}
	return out, warnings, nil
}

// normalizeRubricScoreValue normalizes an already-decoded rubric_score value.
// Cross-schema inconsistencies against normalizedRubric are warnings, not hard
// failures, so legacy score payloads remain recoverable.
func normalizeRubricScoreValue(value any, normalizedRubric map[string]any) (map[string]any, []string, error) {
	var warnings []string
	rubricMaxByID, rubricMaxTotal := rubricMaxScores(normalizedRubric)

	var criteriaScoresValue any
	var totalValue any
	var hasTotal bool
	var maxTotalValue any
	var hasMaxTotal bool
	var summary string
	var confidenceValue any
	var hasConfidence bool

	switch v := value.(type) {
	case map[string]any:
		if scores, _, ok := lookupRubricField(v, "criteria_scores", "criterion_scores", "scores", "score_by_criterion", "per_criterion", "criteria"); ok {
			criteriaScoresValue = scores
		}
		totalValue, _, hasTotal = lookupRubricField(v, "total", "overall", "overall_score", "score")
		maxTotalValue, _, hasMaxTotal = lookupRubricField(v, "max_total", "maxTotal", "max_score", "maxScore", "total_possible")
		if s, ok := firstRubricString(v, "summary", "feedback", "notes", "explanation"); ok {
			summary = s
		}
		confidenceValue, _, hasConfidence = lookupRubricField(v, "confidence")
		if criteriaScoresValue == nil {
			legacy := legacyRubricScoreMap(v)
			if len(legacy) > 0 {
				criteriaScoresValue = legacy
				warnings = append(warnings, "rubric_score_json has no criteria_scores field; normalized non-metadata object keys as criteria_scores")
			}
		}
	case []any:
		criteriaScoresValue = v
		warnings = append(warnings, "rubric_score_json top-level array normalized as criteria_scores")
	default:
		score, coerced, ok := rubricFiniteNumber(value)
		if !ok || score < 0 {
			return nil, warnings, fmt.Errorf("rubric_score_json must be an object, array, or non-negative numeric score")
		}
		if coerced {
			warnings = append(warnings, "rubric_score_json scalar string coerced to number")
		}
		criteriaScoresValue = []any{map[string]any{"id": "overall", "score": score}}
		warnings = append(warnings, "rubric_score_json scalar normalized as an overall criterion score")
	}

	if criteriaScoresValue == nil {
		if !hasTotal {
			return nil, warnings, fmt.Errorf("rubric_score_json must contain criteria_scores or total")
		}
		score, coerced, ok := rubricFiniteNumber(totalValue)
		if !ok || score < 0 {
			return nil, warnings, fmt.Errorf("rubric_score_json.total must be a non-negative finite number")
		}
		if coerced {
			warnings = append(warnings, "rubric_score_json.total string coerced to number")
		}
		criteriaScoresValue = []any{map[string]any{"id": "overall", "score": score}}
		warnings = append(warnings, "rubric_score_json.total normalized as an overall criterion score")
	}

	scoreItems, err := normalizeRubricScoreCriteria(criteriaScoresValue, &warnings)
	if err != nil {
		return nil, warnings, err
	}
	if len(scoreItems) == 0 {
		return nil, warnings, fmt.Errorf("rubric_score_json.criteria_scores must contain at least one score")
	}

	total := 0.0
	for _, item := range scoreItems {
		total += item.score
	}
	if hasTotal {
		providedTotal, coerced, ok := rubricFiniteNumber(totalValue)
		if !ok || providedTotal < 0 {
			return nil, warnings, fmt.Errorf("rubric_score_json.total must be a non-negative finite number")
		}
		if coerced {
			warnings = append(warnings, "rubric_score_json.total string coerced to number")
		}
		if !rubricFloatEqual(providedTotal, total) {
			warnings = append(warnings, "rubric_score_json.total recomputed from criteria_scores")
		}
	} else {
		warnings = append(warnings, "rubric_score_json.total missing; computed from criteria_scores")
	}

	maxTotal, err := normalizeRubricScoreMaxTotal(scoreItems, maxTotalValue, hasMaxTotal, rubricMaxTotal, &warnings)
	if err != nil {
		return nil, warnings, err
	}
	if maxTotal < total && !rubricFloatEqual(maxTotal, total) {
		warnings = append(warnings, "rubric_score_json.total exceeds max_total")
	}

	outScores := make([]map[string]any, 0, len(scoreItems))
	seenScores := make(map[string]bool, len(scoreItems))
	for _, item := range scoreItems {
		id, _ := item.normalized["id"].(string)
		if seenScores[id] {
			return nil, warnings, fmt.Errorf("rubric_score_json.criteria_scores contains duplicate id %q", id)
		}
		seenScores[id] = true
		if max, ok := rubricMaxByID[id]; ok && item.score > max && !rubricFloatEqual(item.score, max) {
			warnings = append(warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%s].score exceeds rubric_json max_score", id))
		}
		if len(rubricMaxByID) > 0 {
			if _, ok := rubricMaxByID[id]; !ok {
				warnings = append(warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%s].id is not present in rubric_json.criteria", id))
			}
		}
		outScores = append(outScores, item.normalized)
	}
	for id := range rubricMaxByID {
		if !seenScores[id] {
			warnings = append(warnings, fmt.Sprintf("rubric_score_json.criteria_scores missing rubric criterion %q", id))
		}
	}

	out := map[string]any{
		"criteria_scores": outScores,
		"total":           total,
		"max_total":       maxTotal,
	}
	if summary != "" {
		out["summary"] = summary
	}
	if hasConfidence {
		confidence, coerced, ok := rubricFiniteNumber(confidenceValue)
		if !ok || confidence < 0 || confidence > 1 {
			return nil, warnings, fmt.Errorf("rubric_score_json.confidence must be a finite number in [0, 1]")
		}
		if coerced {
			warnings = append(warnings, "rubric_score_json.confidence string coerced to number")
		}
		out["confidence"] = confidence
	}
	sort.Strings(warnings)
	return out, warnings, nil
}

func parseRubricSchemaJSON(field, raw string) (any, error) {
	if err := validateString(field, raw, maxLongTextLen); err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON: %v", field, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("%s must be valid JSON: %v", field, err)
		}
		return nil, fmt.Errorf("%s must contain a single JSON value", field)
	}
	return parsed, nil
}

func normalizeRubricCriteria(value any, warnings *[]string) ([]map[string]any, float64, error) {
	var criteria []map[string]any
	total := 0.0
	seen := map[string]bool{}

	add := func(criterion map[string]any, maxScore float64) error {
		id, _ := criterion["id"].(string)
		if seen[id] {
			return fmt.Errorf("rubric_json.criteria contains duplicate id %q", id)
		}
		seen[id] = true
		criteria = append(criteria, criterion)
		total += maxScore
		return nil
	}

	switch v := value.(type) {
	case []any:
		for i, item := range v {
			criterion, maxScore, err := normalizeRubricCriterion(item, "", i, warnings)
			if err != nil {
				return nil, 0, err
			}
			if err := add(criterion, maxScore); err != nil {
				return nil, 0, err
			}
		}
	case []map[string]any:
		for i, item := range v {
			criterion, maxScore, err := normalizeRubricCriterion(item, "", i, warnings)
			if err != nil {
				return nil, 0, err
			}
			if err := add(criterion, maxScore); err != nil {
				return nil, 0, err
			}
		}
	case map[string]any:
		*warnings = append(*warnings, "rubric_json.criteria object normalized as criteria array")
		keys := sortedRubricKeys(v)
		for i, key := range keys {
			criterion, maxScore, err := normalizeRubricCriterion(v[key], key, i, warnings)
			if err != nil {
				return nil, 0, err
			}
			if err := add(criterion, maxScore); err != nil {
				return nil, 0, err
			}
		}
	default:
		return nil, 0, fmt.Errorf("rubric_json.criteria must be an array or object")
	}

	return criteria, total, nil
}

func normalizeRubricCriterion(value any, fallbackID string, index int, warnings *[]string) (map[string]any, float64, error) {
	switch v := value.(type) {
	case map[string]any:
		return normalizeRubricCriterionObject(v, fallbackID, index, warnings)
	case string:
		idSource := v
		if fallbackID != "" {
			idSource = fallbackID
		}
		id := normalizedRubricIDOrFallback(idSource, fallbackID, index, warnings, fmt.Sprintf("rubric_json.criteria[%d].id", index))
		item := map[string]any{
			"id":          id,
			"description": strings.TrimSpace(v),
			"max_score":   1.0,
		}
		*warnings = append(*warnings, fmt.Sprintf("rubric_json.criteria[%d] string normalized with max_score=1", index))
		return item, 1, nil
	default:
		maxScore, coerced, ok := rubricFiniteNumber(v)
		if !ok || maxScore <= 0 {
			return nil, 0, fmt.Errorf("rubric_json.criteria[%d] must be an object, string, or positive numeric max_score", index)
		}
		if coerced {
			*warnings = append(*warnings, fmt.Sprintf("rubric_json.criteria[%d] string coerced to max_score", index))
		}
		id := normalizedRubricIDOrFallback("", fallbackID, index, warnings, fmt.Sprintf("rubric_json.criteria[%d].id", index))
		return map[string]any{"id": id, "max_score": maxScore}, maxScore, nil
	}
}

func normalizeRubricCriterionObject(m map[string]any, fallbackID string, index int, warnings *[]string) (map[string]any, float64, error) {
	rawID, hasID := firstRubricString(m, "id", "criterion_id", "key")
	if !hasID && fallbackID != "" {
		rawID = fallbackID
		hasID = true
	}
	if !hasID {
		if name, ok := firstRubricString(m, "name", "title", "label", "criterion"); ok {
			rawID = name
			hasID = true
		}
	}
	if !hasID {
		if desc, ok := firstRubricString(m, "description", "desc", "details", "prompt"); ok {
			rawID = desc
			hasID = true
		}
	}
	id := normalizedRubricIDOrFallback(rawID, fallbackID, index, warnings, fmt.Sprintf("rubric_json.criteria[%d].id", index))

	maxScore := 1.0
	if rawMax, key, ok := lookupRubricField(m, "max_score", "maxScore", "max", "points", "points_possible", "weight"); ok {
		n, coerced, ok := rubricFiniteNumber(rawMax)
		if !ok || n <= 0 {
			return nil, 0, fmt.Errorf("rubric_json.criteria[%d].max_score must be a positive finite number", index)
		}
		maxScore = n
		if coerced {
			*warnings = append(*warnings, fmt.Sprintf("rubric_json.criteria[%d].max_score string coerced to number", index))
		}
		if key != "max_score" && key != "maxScore" {
			*warnings = append(*warnings, fmt.Sprintf("rubric_json.criteria[%d].%s normalized as max_score", index, key))
		}
	} else {
		*warnings = append(*warnings, fmt.Sprintf("rubric_json.criteria[%d].max_score missing; defaulted to 1", index))
	}

	out := map[string]any{"id": id, "max_score": maxScore}
	if desc, ok := firstRubricString(m, "description", "desc", "details", "prompt"); ok {
		out["description"] = desc
	}
	return out, maxScore, nil
}

type rubricScoreCriterion struct {
	normalized map[string]any
	score      float64
	maxScore   float64
	hasMax     bool
}

func normalizeRubricScoreCriteria(value any, warnings *[]string) ([]rubricScoreCriterion, error) {
	var scores []rubricScoreCriterion
	switch v := value.(type) {
	case []any:
		for i, item := range v {
			score, err := normalizeRubricScoreCriterion(item, "", i, warnings)
			if err != nil {
				return nil, err
			}
			scores = append(scores, score)
		}
	case []map[string]any:
		for i, item := range v {
			score, err := normalizeRubricScoreCriterion(item, "", i, warnings)
			if err != nil {
				return nil, err
			}
			scores = append(scores, score)
		}
	case map[string]any:
		*warnings = append(*warnings, "rubric_score_json.criteria_scores object normalized as criteria_scores array")
		keys := sortedRubricKeys(v)
		for i, key := range keys {
			score, err := normalizeRubricScoreCriterion(v[key], key, i, warnings)
			if err != nil {
				return nil, err
			}
			scores = append(scores, score)
		}
	default:
		return nil, fmt.Errorf("rubric_score_json.criteria_scores must be an array or object")
	}
	return scores, nil
}

func normalizeRubricScoreCriterion(value any, fallbackID string, index int, warnings *[]string) (rubricScoreCriterion, error) {
	switch v := value.(type) {
	case map[string]any:
		return normalizeRubricScoreCriterionObject(v, fallbackID, index, warnings)
	default:
		score, coerced, ok := rubricFiniteNumber(v)
		if !ok || score < 0 {
			return rubricScoreCriterion{}, fmt.Errorf("rubric_score_json.criteria_scores[%d].score must be a non-negative finite number", index)
		}
		if coerced {
			*warnings = append(*warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].score string coerced to number", index))
		}
		id := normalizedRubricIDOrFallback("", fallbackID, index, warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].id", index))
		return rubricScoreCriterion{
			normalized: map[string]any{"id": id, "score": score},
			score:      score,
		}, nil
	}
}

func normalizeRubricScoreCriterionObject(m map[string]any, fallbackID string, index int, warnings *[]string) (rubricScoreCriterion, error) {
	rawID, hasID := firstRubricString(m, "id", "criterion_id", "key", "name", "criterion")
	if !hasID && fallbackID != "" {
		rawID = fallbackID
	}
	id := normalizedRubricIDOrFallback(rawID, fallbackID, index, warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].id", index))

	rawScore, _, ok := lookupRubricField(m, "score", "value", "points", "earned", "awarded")
	if !ok {
		return rubricScoreCriterion{}, fmt.Errorf("rubric_score_json.criteria_scores[%d].score is required", index)
	}
	score, coerced, ok := rubricFiniteNumber(rawScore)
	if !ok || score < 0 {
		return rubricScoreCriterion{}, fmt.Errorf("rubric_score_json.criteria_scores[%d].score must be a non-negative finite number", index)
	}
	if coerced {
		*warnings = append(*warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].score string coerced to number", index))
	}

	out := map[string]any{"id": id, "score": score}
	if evidence, ok := firstRubricString(m, "evidence", "feedback", "rationale", "explanation", "notes"); ok {
		out["evidence"] = evidence
	}
	if errorType, ok := firstRubricString(m, "error_type", "errorType"); ok {
		out["error_type"] = errorType
		if err := validateEnum("error_type", errorType, allowedErrorTypes); err != nil {
			*warnings = append(*warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].error_type is not canonical: %s", index, errorType))
		}
	}

	item := rubricScoreCriterion{normalized: out, score: score}
	if rawMax, _, ok := lookupRubricField(m, "max_score", "maxScore", "max", "possible", "points_possible"); ok {
		maxScore, coerced, ok := rubricFiniteNumber(rawMax)
		if !ok || maxScore <= 0 {
			return rubricScoreCriterion{}, fmt.Errorf("rubric_score_json.criteria_scores[%d].max_score must be a positive finite number", index)
		}
		if coerced {
			*warnings = append(*warnings, fmt.Sprintf("rubric_score_json.criteria_scores[%d].max_score string coerced to number", index))
		}
		item.maxScore = maxScore
		item.hasMax = true
	}
	return item, nil
}

func normalizeRubricScoreMaxTotal(items []rubricScoreCriterion, rawMaxTotal any, hasMaxTotal bool, rubricMaxTotal float64, warnings *[]string) (float64, error) {
	if hasMaxTotal {
		maxTotal, coerced, ok := rubricFiniteNumber(rawMaxTotal)
		if !ok || maxTotal <= 0 {
			return 0, fmt.Errorf("rubric_score_json.max_total must be a positive finite number")
		}
		if coerced {
			*warnings = append(*warnings, "rubric_score_json.max_total string coerced to number")
		}
		if rubricMaxTotal > 0 {
			if !rubricFloatEqual(maxTotal, rubricMaxTotal) {
				*warnings = append(*warnings, "rubric_score_json.max_total differs from rubric_json criteria max_total; using rubric_json")
			}
			return rubricMaxTotal, nil
		}
		return maxTotal, nil
	}

	if rubricMaxTotal > 0 {
		*warnings = append(*warnings, "rubric_score_json.max_total missing; inferred from rubric_json")
		return rubricMaxTotal, nil
	}

	total := 0.0
	for _, item := range items {
		if item.hasMax {
			total += item.maxScore
			continue
		}
		if item.score <= 1 {
			total += 1
		} else {
			total += item.score
		}
	}
	*warnings = append(*warnings, "rubric_score_json.max_total missing; inferred from criteria_scores")
	return total, nil
}

func rubricMaxScores(normalizedRubric map[string]any) (map[string]float64, float64) {
	out := map[string]float64{}
	if normalizedRubric == nil {
		return out, 0
	}
	criteriaValue, ok := normalizedRubric["criteria"]
	if !ok {
		return out, 0
	}

	add := func(item map[string]any) {
		rawID, _ := item["id"].(string)
		id, ok := normalizeRubricID(rawID)
		if !ok {
			return
		}
		maxScore, _, ok := rubricFiniteNumber(item["max_score"])
		if !ok || maxScore <= 0 {
			return
		}
		out[id] = maxScore
	}

	switch criteria := criteriaValue.(type) {
	case []map[string]any:
		for _, item := range criteria {
			add(item)
		}
	case []any:
		for _, raw := range criteria {
			if item, ok := raw.(map[string]any); ok {
				add(item)
			}
		}
	}

	total := 0.0
	for _, maxScore := range out {
		total += maxScore
	}
	return out, total
}

func legacyRubricCriteriaMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if isRubricMetaKey(k) {
			continue
		}
		out[k] = v
	}
	return out
}

func legacyRubricScoreMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if isRubricScoreMetaKey(k) {
			continue
		}
		out[k] = v
	}
	return out
}

func isRubricMetaKey(key string) bool {
	switch strings.ToLower(key) {
	case "criteria", "passing_score", "passingscore", "pass_score", "passscore", "threshold", "scale", "summary", "instructions", "description", "title", "name":
		return true
	default:
		return false
	}
}

func isRubricScoreMetaKey(key string) bool {
	switch strings.ToLower(key) {
	case "criteria_scores", "criterion_scores", "scores", "score_by_criterion", "per_criterion", "criteria", "total", "overall", "overall_score", "score", "max_total", "maxtotal", "max_score", "maxscore", "total_possible", "summary", "feedback", "notes", "explanation", "confidence", "passed", "pass":
		return true
	default:
		return false
	}
}

func lookupRubricField(m map[string]any, names ...string) (any, string, bool) {
	for _, name := range names {
		if v, ok := m[name]; ok {
			return v, name, true
		}
	}
	for _, key := range sortedRubricKeys(m) {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				return m[key], key, true
			}
		}
	}
	return nil, "", false
}

func firstRubricString(m map[string]any, names ...string) (string, bool) {
	raw, _, ok := lookupRubricField(m, names...)
	if !ok {
		return "", false
	}
	s, ok := raw.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	return s, s != ""
}

func rubricFiniteNumber(value any) (float64, bool, bool) {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Float64()
		return n, false, err == nil && rubricFinite(n)
	case float64:
		return v, false, rubricFinite(v)
	case float32:
		n := float64(v)
		return n, false, rubricFinite(n)
	case int:
		return float64(v), false, true
	case int8:
		return float64(v), false, true
	case int16:
		return float64(v), false, true
	case int32:
		return float64(v), false, true
	case int64:
		return float64(v), false, true
	case uint:
		return float64(v), false, true
	case uint8:
		return float64(v), false, true
	case uint16:
		return float64(v), false, true
	case uint32:
		return float64(v), false, true
	case uint64:
		return float64(v), false, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, true, err == nil && rubricFinite(n)
	default:
		return 0, false, false
	}
}

func rubricFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func rubricFloatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func normalizedRubricIDOrFallback(raw, fallback string, index int, warnings *[]string, field string) string {
	source := raw
	if strings.TrimSpace(source) == "" {
		source = fallback
	}
	id, ok := normalizeRubricID(source)
	if !ok {
		id = fmt.Sprintf("criterion_%d", index+1)
		*warnings = append(*warnings, fmt.Sprintf("%s missing; generated %q", field, id))
		return id
	}
	if strings.TrimSpace(source) != id {
		*warnings = append(*warnings, fmt.Sprintf("%s normalized to %q", field, id))
	}
	return id
}

func normalizeRubricID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	var b strings.Builder
	lastSep := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastSep = false
			continue
		}
		if b.Len() > 0 && !lastSep {
			b.WriteByte('_')
			lastSep = true
		}
	}
	out := strings.Trim(b.String(), "_")
	return out, out != ""
}

func sortedRubricKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
