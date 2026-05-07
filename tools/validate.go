// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"fmt"
	"math"

	"tutor-mcp/models"
)

// validateConceptInDomain checks that concept is a member of d.Graph.Concepts.
// It is the shared write-side guard for record_interaction, submit_answer, and
// any other tool that mutates a learner's cognitive state on a per-concept
// basis. The error string mirrors pick_concept's read-side guard so the LLM
// can self-correct uniformly across read and write surfaces.
func validateConceptInDomain(d *models.Domain, concept string) error {
	if d == nil {
		return fmt.Errorf("no active domain — call init_domain first")
	}
	for _, c := range d.Graph.Concepts {
		if c == concept {
			return nil
		}
	}
	return fmt.Errorf(
		"concept %q is not part of domain %q (call get_learner_context to see the concept list)",
		concept, d.Name,
	)
}

// validateUnitInterval rejects NaN/Inf and any value outside [0, 1].
// Used for probabilities & confidence-style scores fed into BKT/FSRS/IRT
// chains where silent clamping has historically corrupted estimator state
// (issue #25). The error names the offending field so the calling LLM
// can self-correct.
func validateUnitInterval(field string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number in [0, 1] (got non-finite value)", field)
	}
	if v < 0 || v > 1 {
		return fmt.Errorf("%s must be in [0, 1] (got %v)", field, v)
	}
	return nil
}

// validateLikertInt rejects integer Likert ratings outside [min, max].
// Use min=1, max=4 for the affect scale and min=1, max=5 for the
// calibration self-assessment scale. A value of 0 is treated as "not
// provided" by the upstream omitempty tag and must be allowed through.
func validateLikertInt(field string, v, min, max int) error {
	if v == 0 {
		return nil // omitted / not provided
	}
	if v < min || v > max {
		return fmt.Errorf("%s must be in [%d, %d] (got %d)", field, min, max, v)
	}
	return nil
}

// validateLikertFloat rejects NaN/Inf and float Likert ratings outside
// [min, max]. Used by calibration_check whose PredictedMastery field is
// a float64 1-5 scale.
func validateLikertFloat(field string, v, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number in [%v, %v] (got non-finite value)", field, min, max)
	}
	if v < min || v > max {
		return fmt.Errorf("%s must be in [%v, %v] (got %v)", field, min, max, v)
	}
	return nil
}

// validateNonNegativeDuration rejects NaN/Inf, negative values, and
// values exceeding maxSeconds. Used for response_time_seconds where
// negative or absurdly large values were silently feeding the FSRS
// stability/difficulty update.
func validateNonNegativeDuration(field string, v float64, maxSeconds float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number in [0, %v] seconds (got non-finite value)", field, maxSeconds)
	}
	if v < 0 || v > maxSeconds {
		return fmt.Errorf("%s must be in [0, %v] seconds (got %v)", field, maxSeconds, v)
	}
	return nil
}

// validateNonNegativeCount rejects negative integer counts and values
// exceeding max. Used for hints_requested.
func validateNonNegativeCount(field string, v, max int) error {
	if v < 0 || v > max {
		return fmt.Errorf("%s must be in [0, %d] (got %d)", field, max, v)
	}
	return nil
}
