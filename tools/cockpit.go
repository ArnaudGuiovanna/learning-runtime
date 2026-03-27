package tools

import (
	"context"
	"fmt"
	"time"

	"learning-runtime/algorithms"
	"learning-runtime/engine"
	"learning-runtime/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetCockpitStateParams struct{}

type conceptProgress struct {
	Concept    string  `json:"concept"`
	Mastery    float64 `json:"mastery"`
	Retention  float64 `json:"retention"`
	Status     string  `json:"status"`
	CardState  string  `json:"card_state"`
}

func registerGetCockpitState(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_cockpit_state",
		Description: "Genere l'etat complet du cockpit: progression par concept, alertes, signaux, prochaine action.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params GetCockpitStateParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		domain, err := deps.Store.GetDomainByLearner(learnerID)
		if err != nil {
			r, _ := errorResult("aucun domaine configure")
			return r, nil, nil
		}

		states, _ := deps.Store.GetConceptStatesByLearner(learnerID)
		interactions, _ := deps.Store.GetRecentInteractionsByLearner(learnerID, 20)
		sessionStart, _ := deps.Store.GetSessionStart(learnerID)

		// Build mastery map
		mastery := make(map[string]float64)
		stateMap := make(map[string]*models.ConceptState)
		for _, cs := range states {
			mastery[cs.Concept] = cs.PMastery
			stateMap[cs.Concept] = cs
		}

		graph := algorithms.KSTGraph{
			Concepts:      domain.Graph.Concepts,
			Prerequisites: domain.Graph.Prerequisites,
		}

		// Build per-concept progress
		var concepts []conceptProgress
		masteredCount := 0
		for _, concept := range domain.Graph.Concepts {
			status := algorithms.ConceptStatus(graph, mastery, concept)
			cs := stateMap[concept]

			cp := conceptProgress{
				Concept:   concept,
				Mastery:   mastery[concept],
				Retention: 1.0,
				Status:    status,
				CardState: "new",
			}

			if cs != nil {
				cp.CardState = cs.CardState
				elapsed := cs.ElapsedDays
				if cs.LastReview != nil {
					elapsed = int(time.Since(*cs.LastReview).Hours() / 24)
				}
				cp.Retention = algorithms.Retrievability(elapsed, cs.Stability)
			}

			if status == "done" {
				masteredCount++
			}

			concepts = append(concepts, cp)
		}

		// Compute alerts
		alerts := engine.ComputeAlerts(states, interactions, sessionStart)
		if alerts == nil {
			alerts = []models.Alert{}
		}

		// Retention alerts (concepts with retention < 50%)
		var retentionAlerts []map[string]interface{}
		for _, cp := range concepts {
			if cp.Retention < 0.50 && cp.CardState != "new" {
				color := "orange"
				if cp.Retention < 0.30 {
					color = "rouge"
				}
				retentionAlerts = append(retentionAlerts, map[string]interface{}{
					"concept":   cp.Concept,
					"retention": cp.Retention,
					"color":     color,
				})
			}
		}
		if retentionAlerts == nil {
			retentionAlerts = []map[string]interface{}{}
		}

		// Trajectory signal
		signal := "stable"
		if len(interactions) >= 3 {
			recentSuccesses := 0
			for _, i := range interactions[:min(5, len(interactions))] {
				if i.Success {
					recentSuccesses++
				}
			}
			rate := float64(recentSuccesses) / float64(min(5, len(interactions)))
			if rate >= 0.8 {
				signal = "positive"
			} else if rate < 0.4 {
				signal = "declining"
			}
		}

		// Next action
		frontier := algorithms.ComputeFrontier(graph, mastery)
		nextAction := "continuer la revision"
		if len(alerts) > 0 && alerts[0].Urgency == models.UrgencyCritical {
			nextAction = fmt.Sprintf("urgence: %s — %s", alerts[0].Concept, alerts[0].RecommendedAction)
		} else if len(frontier) > 0 {
			nextAction = fmt.Sprintf("nouveau concept: %s", frontier[0])
		}

		// Overall progress
		totalConcepts := len(domain.Graph.Concepts)
		progressPct := 0.0
		if totalConcepts > 0 {
			progressPct = float64(masteredCount) / float64(totalConcepts) * 100
		}

		r, _ := jsonResult(map[string]interface{}{
			"domain":           domain.Name,
			"total_concepts":   totalConcepts,
			"mastered_count":   masteredCount,
			"progress_percent": progressPct,
			"concepts":         concepts,
			"retention_alerts": retentionAlerts,
			"alerts":           alerts,
			"signal":           signal,
			"next_action":      nextAction,
		})
		return r, nil, nil
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
