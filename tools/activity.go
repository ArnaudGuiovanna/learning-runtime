package tools

import (
	"context"

	"learning-runtime/algorithms"
	"learning-runtime/engine"
	"learning-runtime/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetNextActivityParams struct{}

func registerGetNextActivity(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_next_activity",
		Description: "Determine la prochaine activite optimale pour l'apprenant selon son etat cognitif.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetNextActivityParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// Check if domain exists
		domain, err := deps.Store.GetDomainByLearner(learnerID)
		if err != nil || domain == nil {
			r, _ := jsonResult(map[string]interface{}{
				"needs_domain_setup": true,
				"activity": models.Activity{
					Type:         models.ActivitySetupDomain,
					Rationale:    "aucun domaine configure",
					PromptForLLM: "L'apprenant n'a pas encore de domaine. Analyse son objectif, decompose-le en concepts et appelle init_domain().",
				},
			})
			return r, nil, nil
		}

		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		interactions, _ := deps.Store.GetRecentInteractionsByLearner(learnerID, 20)
		sessionStart, _ := deps.Store.GetSessionStart(learnerID)

		// Compute alerts
		alerts := engine.ComputeAlerts(states, interactions, sessionStart)

		// Build mastery map for KST frontier
		mastery := make(map[string]float64)
		for _, cs := range states {
			mastery[cs.Concept] = cs.PMastery
		}

		// Compute frontier
		graph := algorithms.KSTGraph{
			Concepts:      domain.Graph.Concepts,
			Prerequisites: domain.Graph.Prerequisites,
		}
		frontier := algorithms.ComputeFrontier(graph, mastery)

		// Route to next activity
		activity := engine.Route(alerts, frontier, states, interactions, "")

		r, _ := jsonResult(map[string]interface{}{
			"needs_domain_setup": false,
			"activity":           activity,
		})
		return r, nil, nil
	})
}
