// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"
	"time"

	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SubmitAnswerParams holds the inputs for the submit_answer tool.
type SubmitAnswerParams struct {
	Answer        string `json:"answer" jsonschema:"Réponse de l'apprenant à l'exercice."`
	Concept       string `json:"concept" jsonschema:"Concept de l'exercice (renvoyé par request_exercise)."`
	ActivityType  string `json:"activity_type" jsonschema:"Type d'activité (renvoyé par request_exercise)."`
	CalibrationID string `json:"calibration_id,omitempty" jsonschema:"ID de la prédiction calibration_check (optionnel)."`
	DomainID      string `json:"domain_id,omitempty" jsonschema:"Domaine (défaut : dernier actif)."`
}

func registerSubmitAnswer(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "submit_answer",
		Description: "Évalue la réponse de l'apprenant à un exercice (sampling), met à jour BKT/FSRS/IRT (record_interaction), et retourne un payload structuredContent {screen:'feedback', correct, explanation}.",
		Meta:        appUIMeta(),
	}, func(ctx context.Context, req *mcp.CallToolRequest, params SubmitAnswerParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		domain, err := resolveDomain(deps.Store, learnerID, params.DomainID)
		if err != nil || domain == nil {
			r, _ := errorResult("domain not found")
			return r, nil, nil
		}

		chatMode, _ := deps.Store.GetChatModeEnabled(learnerID)

		evalSystem := "Tu évalues une réponse d'apprenant. Retourne strictement un JSON: {\"correct\": bool, \"explanation\": string, \"error_type\"?: string}. Pas de texte hors JSON."
		evalUser := fmt.Sprintf("Concept: %s. Type d'activité: %s. Réponse de l'apprenant: %s", params.Concept, params.ActivityType, params.Answer)

		eval, evalErr := evalWithRetry(ctx, req, evalSystem, evalUser)

		out := map[string]any{
			"screen":  "feedback",
			"concept": params.Concept,
		}

		if evalErr != nil {
			out["mode"] = "fallback_b"
			out["parsed_failed"] = true
			if chatMode {
				out["chat_mode"] = true
				r, _ := textOnlyResult(out)
				return r, nil, nil // nil structuredContent on purpose
			}
			r, _ := jsonResult(out)
			return r, out, nil
		}

		// Persist interaction and run BKT/FSRS/IRT/PFA update chain.
		if _, applyErr := applyInteraction(deps, learnerID, interactionInput{
			Concept:      params.Concept,
			ActivityType: params.ActivityType,
			Success:      eval.Correct,
			ErrorType:    eval.ErrorType,
			CalibrationID: params.CalibrationID,
			// Confidence/HintsRequested/SelfInitiated default to 0/false —
			// submit_answer (iframe path) does not surface them yet.
		}, time.Now().UTC()); applyErr != nil {
			deps.Logger.Error("submit_answer: applyInteraction", "err", applyErr, "learner", learnerID)
			// Non-blocking: still return the feedback to the learner.
		}

		// Optional calibration recording: if the client sent a calibration_id,
		// resolve the prediction and complete it with the actual outcome.
		if params.CalibrationID != "" {
			if calErr := recordCalibrationForAnswer(deps, params.CalibrationID, eval.Correct); calErr != nil {
				deps.Logger.Warn("submit_answer: calibration recording failed", "err", calErr, "learner", learnerID)
				// Non-blocking.
			}
		}

		out["correct"] = eval.Correct
		out["explanation"] = eval.Explanation
		if eval.ErrorType != "" {
			out["error_type"] = eval.ErrorType
		}

		if chatMode {
			out["chat_mode"] = true
			r, _ := textOnlyResult(out)
			return r, nil, nil // nil structuredContent on purpose
		}

		r, _ := jsonResult(out)
		return r, out, nil
	})
}

// evalWithRetry calls sampling once; on parse failure, retries once with a
// stricter prompt. Returns the parse error to the caller after both attempts
// fail.
func evalWithRetry(ctx context.Context, req *mcp.CallToolRequest, systemPrompt, userPrompt string) (engine.EvalResponse, error) {
	raw, err := callSampling(ctx, req, systemPrompt, userPrompt, 400)
	if err != nil {
		return engine.EvalResponse{}, err
	}
	parsed, err := engine.ParseEvalResponse(raw)
	if err == nil {
		return parsed, nil
	}
	// Retry with stricter formatting instruction.
	stricter := systemPrompt + " IMPORTANT: aucun caractère hors du JSON, pas de markdown, pas de ```."
	raw2, err2 := callSampling(ctx, req, stricter, userPrompt, 400)
	if err2 != nil {
		return engine.EvalResponse{}, err2
	}
	return engine.ParseEvalResponse(raw2)
}

// recordCalibrationForAnswer fetches the calibration prediction by ID and
// completes it using the binary outcome (correct → 1.0, wrong → 0.0).
// The delta is computed as predicted − actual, matching the existing
// record_calibration_result tool convention.
func recordCalibrationForAnswer(deps *Deps, calibrationID string, correct bool) error {
	record, err := deps.Store.GetCalibrationRecord(calibrationID)
	if err != nil {
		return fmt.Errorf("get calibration record: %w", err)
	}
	var actual float64
	if correct {
		actual = 1.0
	}
	delta := record.Predicted - actual
	return deps.Store.CompleteCalibrationRecord(calibrationID, actual, delta)
}
