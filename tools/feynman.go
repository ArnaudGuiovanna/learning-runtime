// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"

	"tutor-mcp/algorithms"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FeynmanChallengeParams struct {
	ConceptID string `json:"concept_id" jsonschema:"the concept to explain using the Feynman method"`
	DomainID  string `json:"domain_id,omitempty" jsonschema:"domain ID (optional)"`
}

func registerFeynmanChallenge(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "feynman_challenge",
		Description: "Ask the learner to explain a mastered concept in their own words. The LLM identifies gaps and injects them into the BKT graph.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params FeynmanChallengeParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("feynman_challenge: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.ConceptID == "" {
			r, _ := errorResult("concept_id is required")
			return r, nil, nil
		}

		// String length cap (issue #82). concept_id is echoed into the
		// generated prompt_text via fmt.Sprintf and used to look up state;
		// without this guard a misbehaving caller could push multi-MB
		// strings into orchestrator output.
		if err := validateString("concept_id", params.ConceptID, maxShortLabelLen); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		cs, err := deps.Store.GetConceptState(learnerID, params.ConceptID)
		if err != nil {
			deps.Logger.Error("feynman_challenge: failed to get concept state", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("concept not found: %v", err))
			return r, nil, nil
		}

		bktState := algorithms.BKTState{PMastery: cs.PMastery}
		if !algorithms.BKTIsMastered(bktState) {
			r, _ := jsonResult(map[string]interface{}{
				"eligible":  false,
				"mastery":   cs.PMastery,
				"threshold": algorithms.MasteryBKT(),
				"message":   "Concept not yet mastered. Keep up the regular practice.",
			})
			return r, nil, nil
		}

		promptText := fmt.Sprintf(
			"Explain the concept '%s' as if you were teaching it to someone who knows nothing about it. "+
				"No technical jargon — use analogies and concrete examples. "+
				"The goal is to verify that you truly understood it, not just that you can recite it.\n\n"+
				"After your explanation, I will identify the fuzzy or incomplete points "+
				"and turn them into micro-concepts to work on.",
			params.ConceptID,
		)

		r, _ := jsonResult(map[string]interface{}{
			"eligible":    true,
			"prompt_text": promptText,
			"concept_id":  params.ConceptID,
			"instructions_for_llm": "After the learner's explanation, identify the specific conceptual gaps. " +
				"For each gap, generate a short label and a description. " +
				"Ask the learner for confirmation before injecting the gaps into the graph via add_concepts(). " +
				"The new micro-concepts must have the source concept as a prerequisite.",
			"gap_template": map[string]interface{}{
				"label":          "<nom court du gap>",
				"description":    "<ce qui manque dans l'explication>",
				"initial_pl0":    0.1,
				"source_concept": params.ConceptID,
			},
		})
		return r, nil, nil
	})
}
