// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"

	"tutor-mcp/algorithms"
	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TransferChallengeParams struct {
	ConceptID   string `json:"concept_id" jsonschema:"the concept to test in transfer"`
	ContextType string `json:"context_type,omitempty" jsonschema:"context type: real_world, interview, teaching, debugging, creative (optional)"`
	DomainID    string `json:"domain_id,omitempty" jsonschema:"domain ID (optional)"`
}

func registerTransferChallenge(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "transfer_challenge",
		Description: "Generate a novel situation to test the transfer of a mastered concept outside its original context.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params TransferChallengeParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("transfer_challenge: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.ConceptID == "" {
			r, _ := errorResult("concept_id is required")
			return r, nil, nil
		}

		// String length caps (issue #82). concept_id flows into the
		// transfer_records query / response and context_type ends up in the
		// persisted row label — without these guards a misbehaving caller
		// could push multi-MB strings into the read path and bloat downstream
		// telemetry.
		stringFields := []struct {
			name  string
			value string
			max   int
		}{
			{"concept_id", params.ConceptID, maxShortLabelLen},
			{"context_type", params.ContextType, maxShortLabelLen},
		}
		for _, f := range stringFields {
			if err := validateString(f.name, f.value, f.max); err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
		}

		cs, err := deps.Store.GetConceptState(learnerID, params.ConceptID)
		if err != nil {
			deps.Logger.Error("transfer_challenge: failed to get concept state", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("concept not found: %v", err))
			return r, nil, nil
		}

		bktState := algorithms.BKTState{PMastery: cs.PMastery}
		if !algorithms.BKTIsMastered(bktState) {
			r, _ := jsonResult(map[string]interface{}{
				"eligible": false,
				"mastery":  cs.PMastery,
				"message":  "Concept not yet mastered. The transfer challenge requires BKT >= 0.85.",
			})
			return r, nil, nil
		}

		contextType := params.ContextType
		if contextType == "" {
			contextType = "real_world"
		}

		existingTransfers, _ := deps.Store.GetTransferScores(learnerID, params.ConceptID)
		var testedContexts []string
		for _, tr := range existingTransfers {
			testedContexts = append(testedContexts, fmt.Sprintf("%s (%.0f%%)", tr.ContextType, tr.Score*100))
		}

		promptText := fmt.Sprintf(
			"Generate a completely new situation that tests the transfer of the concept '%s' "+
				"in a context of type '%s'. "+
				"The situation must NOT resemble previous exercises. "+
				"The goal: verify that the learner can apply this concept in a context they have never seen.\n\n"+
				"After the learner's response, evaluate the transfer_score (0-1) and "+
				"call record_transfer_result with the result.",
			params.ConceptID, contextType,
		)

		r, _ := jsonResult(map[string]interface{}{
			"eligible":        true,
			"concept_id":      params.ConceptID,
			"context_type":    contextType,
			"prompt_text":     promptText,
			"tested_contexts": testedContexts,
		})
		return r, nil, nil
	})
}

// ─── record_transfer_result ─────────────────────────────────────────────────

type RecordTransferResultParams struct {
	ConceptID   string  `json:"concept_id" jsonschema:"the concept being tested"`
	ContextType string  `json:"context_type" jsonschema:"challenge context type: real_world, interview, teaching, debugging, creative"`
	Score       float64 `json:"score" jsonschema:"transfer score between 0 and 1"`
	SessionID   string  `json:"session_id,omitempty" jsonschema:"session ID (optional)"`
	DomainID    string  `json:"domain_id,omitempty" jsonschema:"domain ID (optional)"`
}

// allowedRecordTransferContextTypes enumerates the documented context_type
// values for record_transfer_result. Issue #96: a free-string ContextType
// allowed orphan transfer rows ("banana", "") to pollute TRANSFER_BLOCKED
// alerts. Kept inline here (rather than in tools/validate.go) to avoid a
// merge conflict with the cycle-2 sibling PR that introduces validateEnum.
var allowedRecordTransferContextTypes = []string{
	"real_world", "interview", "teaching", "debugging", "creative",
}

func registerRecordTransferResult(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_transfer_result",
		Description: "Record the result of a transfer challenge.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params RecordTransferResultParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("record_transfer_result: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// String length caps (issue #82). concept_id, context_type and
		// session_id end up in transfer_records rows; without these guards a
		// misbehaving caller could push multi-MB strings into the table.
		stringFields := []struct {
			name  string
			value string
			max   int
		}{
			{"concept_id", params.ConceptID, maxShortLabelLen},
			{"context_type", params.ContextType, maxShortLabelLen},
			{"session_id", params.SessionID, maxShortLabelLen},
		}
		for _, f := range stringFields {
			if err := validateString(f.name, f.value, f.max); err != nil {
				r, _ := errorResult(err.Error())
				return r, nil, nil
			}
		}

		// Score is a unit-interval transfer rating. Reject NaN/Inf and any
		// value outside [0, 1] to keep transfer_score reports & the
		// blocked < 0.50 gate meaningful (issue #25).
		if err := validateUnitInterval("score", params.Score); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// Enum guard for context_type. Without this, hallucinated labels
		// ("banana", "") flowed into the transfer_records table and
		// poisoned downstream TRANSFER_BLOCKED alerts (issue #96).
		ctOK := false
		for _, v := range allowedRecordTransferContextTypes {
			if params.ContextType == v {
				ctOK = true
				break
			}
		}
		if !ctOK {
			r, _ := errorResult(fmt.Sprintf(
				"context_type %q is invalid: must be one of: real_world, interview, teaching, debugging, creative",
				params.ContextType,
			))
			return r, nil, nil
		}

		// Resolve the active domain (honoring the optional domain_id) and
		// validate the concept against its concept list. Without this guard
		// the tool silently inserts orphan transfer rows for hallucinated
		// or stale concept names — see issue #96.
		domain, err := resolveDomain(deps.Store, learnerID, params.DomainID)
		if err != nil || domain == nil {
			if params.DomainID != "" {
				deps.Logger.Error("record_transfer_result: domain not found by id", "err", err, "learner", learnerID, "domain_id", params.DomainID)
				r, _ := errorResult("domain not found")
				return r, nil, nil
			}
			deps.Logger.Info("record_transfer_result: no active domain - needs setup", "learner", learnerID)
			r, _ := noActiveDomainResult()
			return r, nil, nil
		}
		if err := validateConceptInDomain(domain, params.ConceptID); err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		record := &models.TransferRecord{
			LearnerID:   learnerID,
			ConceptID:   params.ConceptID,
			ContextType: params.ContextType,
			Score:       params.Score,
			SessionID:   params.SessionID,
		}

		if err := deps.Store.CreateTransferRecord(record); err != nil {
			deps.Logger.Error("record_transfer_result: failed to create transfer record", "err", err, "learner", learnerID)
			r, _ := errorResult(fmt.Sprintf("failed to record transfer: %v", err))
			return r, nil, nil
		}

		r, _ := jsonResult(map[string]interface{}{
			"recorded":       true,
			"transfer_score": params.Score,
			"blocked":        params.Score < 0.50,
		})
		return r, nil, nil
	})
}
