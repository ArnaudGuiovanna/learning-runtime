package tools

import (
	"context"
	"fmt"
	"time"

	"learning-runtime/algorithms"
	"learning-runtime/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RecordInteractionParams struct {
	Concept             string  `json:"concept" jsonschema:"description=Le concept concerne"`
	ActivityType        string  `json:"activity_type" jsonschema:"description=Type d'activite (RECALL_EXERCISE, NEW_CONCEPT, etc.)"`
	Success             bool    `json:"success" jsonschema:"description=L'exercice a ete reussi"`
	ResponseTimeSeconds float64 `json:"response_time_seconds" jsonschema:"description=Temps de reponse en secondes"`
	Confidence          float64 `json:"confidence" jsonschema:"description=Confiance estimee entre 0 et 1"`
	Notes               string  `json:"notes" jsonschema:"description=Notes optionnelles sur l'interaction"`
}

func registerRecordInteraction(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_interaction",
		Description: "Enregistre le resultat d'un exercice et met a jour l'etat cognitif de l'apprenant.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params RecordInteractionParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.Concept == "" {
			r, _ := errorResult("concept is required")
			return r, nil, nil
		}

		// Create interaction record
		interaction := &models.Interaction{
			LearnerID:    learnerID,
			Concept:      params.Concept,
			ActivityType: params.ActivityType,
			Success:      params.Success,
			ResponseTime: int(params.ResponseTimeSeconds),
			Confidence:   params.Confidence,
			Notes:        params.Notes,
		}
		if err := deps.Store.CreateInteraction(interaction); err != nil {
			r, _ := errorResult(fmt.Sprintf("failed to create interaction: %v", err))
			return r, nil, nil
		}

		// Get or create concept state
		cs, err := deps.Store.GetConceptState(learnerID, params.Concept)
		if err != nil {
			cs = models.NewConceptState(learnerID, params.Concept)
		}

		// BKT update
		bktState := algorithms.BKTState{
			PMastery: cs.PMastery,
			PLearn:   cs.PLearn,
			PForget:  cs.PForget,
			PSlip:    cs.PSlip,
			PGuess:   cs.PGuess,
		}
		bktState = algorithms.BKTUpdate(bktState, params.Success)
		cs.PMastery = bktState.PMastery

		// FSRS ReviewCard
		rating := algorithms.Good
		if !params.Success {
			rating = algorithms.Again
		} else if params.Confidence >= 0.9 {
			rating = algorithms.Easy
		} else if params.Confidence < 0.5 {
			rating = algorithms.Hard
		}

		var lastReview time.Time
		if cs.LastReview != nil {
			lastReview = *cs.LastReview
		}
		fsrsCard := algorithms.FSRSCard{
			Stability:     cs.Stability,
			Difficulty:    cs.Difficulty,
			ElapsedDays:   cs.ElapsedDays,
			ScheduledDays: cs.ScheduledDays,
			Reps:          cs.Reps,
			Lapses:        cs.Lapses,
			State:         algorithms.CardState(cs.CardState),
			LastReview:    lastReview,
		}
		now := time.Now().UTC()
		fsrsCard = algorithms.ReviewCard(fsrsCard, rating, now)
		cs.Stability = fsrsCard.Stability
		cs.Difficulty = fsrsCard.Difficulty
		cs.ElapsedDays = fsrsCard.ElapsedDays
		cs.ScheduledDays = fsrsCard.ScheduledDays
		cs.Reps = fsrsCard.Reps
		cs.Lapses = fsrsCard.Lapses
		cs.CardState = string(fsrsCard.State)
		cs.LastReview = &now
		nextReview := now.Add(time.Duration(fsrsCard.ScheduledDays) * 24 * time.Hour)
		cs.NextReview = &nextReview

		// IRT UpdateTheta
		item := algorithms.IRTItem{
			Difficulty:     cs.Difficulty / 10.0, // normalize to IRT scale
			Discrimination: 1.0,
		}
		cs.Theta = algorithms.IRTUpdateTheta(cs.Theta, []algorithms.IRTItem{item}, []bool{params.Success})

		// PFA Update
		pfaState := algorithms.PFAState{
			Successes: cs.PFASuccesses,
			Failures:  cs.PFAFailures,
		}
		pfaState = algorithms.PFAUpdate(pfaState, params.Success)
		cs.PFASuccesses = pfaState.Successes
		cs.PFAFailures = pfaState.Failures

		// Persist updated concept state
		if err := deps.Store.UpsertConceptState(cs); err != nil {
			r, _ := errorResult(fmt.Sprintf("failed to update concept state: %v", err))
			return r, nil, nil
		}

		// Update last active
		_ = deps.Store.UpdateLastActive(learnerID)

		// Compute engagement signal
		engagementSignal := "stable"
		if params.Confidence >= 0.8 && params.Success {
			engagementSignal = "positive"
		} else if !params.Success && params.Confidence < 0.3 {
			engagementSignal = "declining"
		}

		nextReviewHours := float64(fsrsCard.ScheduledDays) * 24.0

		r, _ := jsonResult(map[string]interface{}{
			"updated":              true,
			"new_mastery":          cs.PMastery,
			"next_review_in_hours": nextReviewHours,
			"engagement_signal":    engagementSignal,
		})
		return r, nil, nil
	})
}
