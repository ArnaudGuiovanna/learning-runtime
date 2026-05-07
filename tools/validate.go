// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"fmt"

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
