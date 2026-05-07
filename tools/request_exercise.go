// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"time"

	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RequestExerciseParams struct {
	Concept  string `json:"concept,omitempty" jsonschema:"Concept à travailler. Vide = l'orchestrateur choisit."`
	DomainID string `json:"domain_id,omitempty" jsonschema:"Domaine (défaut : dernier actif)."`
}

func registerRequestExercise(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "request_exercise",
		Description: "Génère et présente le prochain exercice dans l'iframe du Tutor MCP. Appelle l'orchestrateur pour choisir un concept + un type d'activité, puis demande au LLM hôte (sampling) l'énoncé. Retourne un payload structuredContent {screen:'exercise', ...}.",
		Meta:        appUIMeta(),
	}, func(ctx context.Context, req *mcp.CallToolRequest, params RequestExerciseParams) (*mcp.CallToolResult, any, error) {
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

		activity, err := engine.Orchestrate(deps.Store, engine.OrchestratorInput{
			LearnerID: learnerID,
			DomainID:  domain.ID,
			Now:       time.Now().UTC(),
			Config:    engine.NewDefaultPhaseConfig(),
		})
		if err != nil {
			deps.Logger.Error("request_exercise: orchestrate failed", "err", err)
			r, _ := errorResult("could not compute next activity")
			return r, nil, nil
		}

		systemPrompt := "Tu génères l'énoncé d'un exercice pédagogique. Retourne uniquement l'énoncé, sans préface, sans solution, sans hints inline."
		userPrompt := activity.PromptForLLM
		text, samplingErr := callSampling(ctx, req, systemPrompt, userPrompt, 800)

		exercise := map[string]any{
			"concept":         activity.Concept,
			"activity_type":   string(activity.Type),
			"difficulty":      activity.DifficultyTarget,
			"input_kind":      "text",
			"ask_calibration": true,
		}

		isFallback := samplingErr != nil
		if isFallback {
			exercise["prompt_for_llm"] = activity.PromptForLLM
		} else {
			exercise["text"] = text
		}

		out := map[string]any{
			"screen":    "exercise",
			"exercise":  exercise,
			"domain_id": domain.ID,
		}
		if isFallback {
			out["mode"] = "fallback_b"
		}

		r, _ := jsonResult(out)
		return r, out, nil
	})
}
