package tools

import (
	"context"

	"learning-runtime/engine"
	"learning-runtime/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetPendingAlertsParams struct{}

func registerGetPendingAlerts(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_pending_alerts",
		Description: "Recupere les alertes en attente pour l'apprenant. Appeler en premier a chaque reponse.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetPendingAlertsParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		interactions, _ := deps.Store.GetRecentInteractionsByLearner(learnerID, 20)
		sessionStart, _ := deps.Store.GetSessionStart(learnerID)
		alerts := engine.ComputeAlerts(states, interactions, sessionStart)

		hasCritical := false
		for _, a := range alerts {
			if a.Urgency == models.UrgencyCritical {
				hasCritical = true
				break
			}
		}
		if alerts == nil {
			alerts = []models.Alert{}
		}

		r, _ := jsonResult(map[string]interface{}{
			"alerts":       alerts,
			"has_critical": hasCritical,
		})
		return r, nil, nil
	})
}
