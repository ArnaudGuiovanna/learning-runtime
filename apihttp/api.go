// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

// Package apihttp serves the iframe-direct REST endpoints used by the Tutor
// MCP App SPA. The iframe (loaded by claude.ai/Desktop via MCP Apps) cannot
// reliably trigger MCP tool calls back through the host (tools/call returns
// 405 on claude.ai web; ui/message is silently dropped). To deliver the
// "fluide comme une app" UX, the iframe makes direct fetch() requests here
// instead. The iframe is allowed to reach our origin via the connectDomains
// CSP entry on the ui://app resource (see tools/app.go).
package apihttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tutor-mcp/algorithms"
	"tutor-mcp/auth"
	"tutor-mcp/db"
	"tutor-mcp/engine"
	"tutor-mcp/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessions maps learnerID to the active MCP server session that the
// host LLM is connected through. The /api/v1/exercise handler uses this
// to call sampling/createMessage on the session, generating real
// exercise content via the user's Anthropic subscription. Populated by
// the open_app tool handler at iframe-mount time. Last-writer-wins per
// learner — concurrent conversations are rare in practice.
var sessions sync.Map

// RegisterSession is called by tools/app.go's openAppHandler so the
// HTTP API can reach the host LLM via the same session the iframe was
// minted from.
func RegisterSession(learnerID string, sess *mcp.ServerSession) {
	if sess == nil {
		return
	}
	sessions.Store(learnerID, sess)
}

// getSession returns the most recent ServerSession for a learner, or nil.
func getSession(learnerID string) *mcp.ServerSession {
	v, ok := sessions.Load(learnerID)
	if !ok {
		return nil
	}
	s, _ := v.(*mcp.ServerSession)
	return s
}

// Deps holds shared dependencies for the API handlers.
type Deps struct {
	Store   *db.Store
	Logger  *slog.Logger
	BaseURL string
}

// RegisterRoutes wires the /api/v1/* endpoints onto the given mux.
// All endpoints require Authorization: Bearer <jwt>. CORS is permissive
// (Access-Control-Allow-Origin: *) since JWT verification is the security
// boundary.
func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("OPTIONS /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.Handle("GET /api/v1/cockpit", deps.middleware(deps.handleCockpit))
	mux.Handle("POST /api/v1/exercise", deps.middleware(deps.handleExercise))
	mux.Handle("POST /api/v1/submit", deps.middleware(deps.handleSubmit))
	mux.Handle("POST /api/v1/pick_concept", deps.middleware(deps.handlePickConcept))
}

func (d *Deps) middleware(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCORS(w)
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authz, "Bearer ")
		learnerID, err := auth.VerifyJWT(token, d.BaseURL)
		if err != nil {
			d.Logger.Warn("api: invalid jwt", "err", err)
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), auth.LearnerIDKey, learnerID)
		h(w, r.WithContext(ctx))
	})
}

func writeCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
}

func (d *Deps) handleCockpit(w http.ResponseWriter, r *http.Request) {
	learnerID := auth.GetLearnerID(r.Context())
	domainID := r.URL.Query().Get("domain_id")

	graph, err := engine.BuildOLMGraph(d.Store, learnerID, domainID)
	if err != nil {
		d.Logger.Error("api cockpit: build graph", "err", err, "learner", learnerID)
		http.Error(w, `{"error":"could not build cockpit"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, graph)
}

type exerciseReq struct {
	Concept  string `json:"concept,omitempty"`
	DomainID string `json:"domain_id,omitempty"`
}

func (d *Deps) handleExercise(w http.ResponseWriter, r *http.Request) {
	var req exerciseReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	learnerID := auth.GetLearnerID(r.Context())

	domain, err := resolveDomainAPI(d.Store, learnerID, req.DomainID)
	if err != nil || domain == nil {
		http.Error(w, `{"error":"domain not found"}`, http.StatusNotFound)
		return
	}

	activity, err := engine.Orchestrate(d.Store, engine.OrchestratorInput{
		LearnerID: learnerID,
		DomainID:  domain.ID,
		Now:       time.Now().UTC(),
		Config:    engine.NewDefaultPhaseConfig(),
	})
	if err != nil {
		d.Logger.Error("api exercise: orchestrate", "err", err, "learner", learnerID)
		http.Error(w, `{"error":"could not compute next activity"}`, http.StatusInternalServerError)
		return
	}

	// Use session-bridged sampling to generate real exercise content.
	// Falls back gracefully to the raw prompt when no session is available.
	exerciseText := activity.PromptForLLM
	usedSampling := false
	sess := getSession(learnerID)
	if sess == nil {
		d.Logger.Warn("api exercise: no session registered for learner — sampling skipped, will use prompt fallback", "learner", learnerID)
	} else {
		samplingCtx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		resp, err := sess.CreateMessage(samplingCtx, &mcp.CreateMessageParams{
			MaxTokens:    800,
			SystemPrompt: "Tu es un tuteur pédagogique. Génère un exercice pour un apprenant. Retourne uniquement l'énoncé de l'exercice (la consigne et la question), sans préface, sans solution, sans hints inline. Style clair et concis. Markdown autorisé pour mise en forme légère.",
			Messages: []*mcp.SamplingMessage{
				{Role: "user", Content: &mcp.TextContent{Text: activity.PromptForLLM}},
			},
		})
		if err == nil && resp != nil {
			if tc, ok := resp.Content.(*mcp.TextContent); ok && tc.Text != "" {
				exerciseText = tc.Text
				usedSampling = true
			}
			d.Logger.Info("api exercise: sampling ok", "learner", learnerID, "chars", len(exerciseText))
		} else {
			d.Logger.Warn("api exercise: sampling failed, falling back to prompt", "err", err, "learner", learnerID)
		}
	}
	d.Logger.Info("api exercise: returning", "learner", learnerID, "used_sampling", usedSampling, "concept", activity.Concept)

	out := map[string]any{
		"screen": "exercise",
		"exercise": map[string]any{
			"concept":         activity.Concept,
			"activity_type":   string(activity.Type),
			"difficulty":      activity.DifficultyTarget,
			"input_kind":      "text",
			"ask_calibration": true,
			"text":            exerciseText,
		},
		"domain_id": domain.ID,
	}
	writeJSON(w, out)
}

type submitReq struct {
	Answer        string `json:"answer"`
	Concept       string `json:"concept"`
	ActivityType  string `json:"activity_type"`
	PredictedMast int    `json:"predicted_mastery,omitempty"`
	CalibrationID string `json:"calibration_id,omitempty"`
	DomainID      string `json:"domain_id,omitempty"`
}

func (d *Deps) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req submitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	learnerID := auth.GetLearnerID(r.Context())
	if req.Concept == "" || req.ActivityType == "" {
		http.Error(w, `{"error":"concept and activity_type required"}`, http.StatusBadRequest)
		return
	}

	domain, err := resolveDomainAPI(d.Store, learnerID, req.DomainID)
	if err != nil || domain == nil {
		http.Error(w, `{"error":"domain not found"}`, http.StatusNotFound)
		return
	}

	// Heuristic fallback: success = answer is non-empty AND length > 5 chars.
	// The orchestrator's next-step decisions still feed off BKT/FSRS/IRT/PFA —
	// even a coarse signal beats no signal.
	correct := len(strings.TrimSpace(req.Answer)) >= 5

	explanation := "Réponse enregistrée."
	if !correct {
		explanation = "Réponse trop courte ou vide. Reformule en au moins une phrase complète."
	}

	// Use session-bridged sampling for LLM-evaluated feedback when available.
	// Replaces the heuristic length-based check; heuristic stays as fallback.
	sessSubmit := getSession(learnerID)
	if sessSubmit == nil {
		d.Logger.Warn("api submit: no session registered — falling back to length heuristic", "learner", learnerID)
	}
	if sess := sessSubmit; sess != nil {
		samplingCtx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		evalUser := fmt.Sprintf("Concept: %s. Activité: %s. Réponse de l'apprenant: %s",
			req.Concept, req.ActivityType, req.Answer)
		resp, err := sess.CreateMessage(samplingCtx, &mcp.CreateMessageParams{
			MaxTokens:    400,
			SystemPrompt: "Tu évalues la réponse d'un apprenant à un exercice. Retourne strictement un JSON: {\"correct\": bool, \"explanation\": string}. Aucun texte hors JSON.",
			Messages: []*mcp.SamplingMessage{
				{Role: "user", Content: &mcp.TextContent{Text: evalUser}},
			},
		})
		if err == nil && resp != nil {
			if tc, ok := resp.Content.(*mcp.TextContent); ok {
				// Light JSON parse — strip ```json``` fences if present.
				text := strings.TrimSpace(tc.Text)
				if strings.HasPrefix(text, "```") {
					text = strings.TrimPrefix(text, "```json")
					text = strings.TrimPrefix(text, "```")
					text = strings.TrimSuffix(text, "```")
					text = strings.TrimSpace(text)
				}
				var ev struct {
					Correct     bool   `json:"correct"`
					Explanation string `json:"explanation"`
				}
				if jerr := json.Unmarshal([]byte(text), &ev); jerr == nil {
					correct = ev.Correct
					if ev.Explanation != "" {
						explanation = ev.Explanation
					}
				} else {
					d.Logger.Warn("api submit: malformed eval JSON", "err", jerr, "learner", learnerID, "raw", text)
				}
			}
		} else {
			d.Logger.Warn("api submit: sampling failed", "err", err, "learner", learnerID)
		}
	}

	// Apply BKT/FSRS/IRT update via the same algorithm chain used by
	// record_interaction so server state stays consistent with chat-mode.
	if err := applyInteractionAPI(d.Store, learnerID, req.Concept, req.ActivityType, correct, time.Now().UTC()); err != nil {
		d.Logger.Error("api submit: applyInteraction", "err", err, "learner", learnerID)
		// Non-blocking: still return feedback.
	}

	out := map[string]any{
		"screen":      "feedback",
		"correct":     correct,
		"explanation": explanation,
		"concept":     req.Concept,
	}
	writeJSON(w, out)
}

type pickReq struct {
	Concept  string `json:"concept"`
	DomainID string `json:"domain_id,omitempty"`
}

func (d *Deps) handlePickConcept(w http.ResponseWriter, r *http.Request) {
	var req pickReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	learnerID := auth.GetLearnerID(r.Context())
	domain, err := resolveDomainAPI(d.Store, learnerID, req.DomainID)
	if err != nil || domain == nil {
		http.Error(w, `{"error":"domain not found"}`, http.StatusNotFound)
		return
	}

	// Set or clear pinned_concept on the domain.
	// SetPinnedConcept(learnerID, domainID, concept) — ownership is validated
	// inside the store method.
	if err := d.Store.SetPinnedConcept(learnerID, domain.ID, req.Concept); err != nil {
		d.Logger.Error("api pick_concept: store", "err", err)
		http.Error(w, `{"error":"could not pin concept"}`, http.StatusInternalServerError)
		return
	}

	// Return the updated cockpit graph (same shape as /api/v1/cockpit).
	graph, err := engine.BuildOLMGraph(d.Store, learnerID, domain.ID)
	if err != nil {
		d.Logger.Error("api pick_concept: rebuild graph", "err", err)
		http.Error(w, `{"error":"could not rebuild cockpit"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, graph)
}

// resolveDomainAPI mirrors the resolveDomain helper in tools/ but lives here
// so apihttp doesn't depend on the tools package. Pick the explicit domain
// ID if owned by the learner, otherwise fall back to the learner's most
// recent active domain.
func resolveDomainAPI(store *db.Store, learnerID, domainID string) (*models.Domain, error) {
	if domainID != "" {
		d, err := store.GetDomainByID(domainID)
		if err != nil || d == nil {
			return nil, err
		}
		if d.LearnerID != learnerID {
			return nil, nil // foreign domain — refuse
		}
		return d, nil
	}
	// Fall back to most recent active domain.
	domains, err := store.GetDomainsByLearner(learnerID, false /*includeArchived*/)
	if err != nil || len(domains) == 0 {
		return nil, err
	}
	// GetDomainsByLearner already filters archived=0 and orders DESC by created_at.
	return domains[0], nil
}

// applyInteractionAPI is the slim version of applyInteraction in tools/.
// It runs the BKT/FSRS/IRT/PFA update chain directly. We don't import the
// tools/ helper to avoid a circular dependency; the duplication is
// intentional and acceptable here.
func applyInteractionAPI(store *db.Store, learnerID, concept, activityType string, success bool, now time.Time) error {
	// Load or bootstrap concept state.
	cs, err := store.GetConceptState(learnerID, concept)
	if err != nil || cs == nil {
		cs = models.NewConceptState(learnerID, concept)
	}

	// Persist the interaction row.
	interaction := &models.Interaction{
		LearnerID:    learnerID,
		Concept:      concept,
		ActivityType: activityType,
		Success:      success,
		CreatedAt:    now,
	}
	if err := store.CreateInteraction(interaction); err != nil {
		return err
	}

	// BKT update.
	bktState := algorithms.BKTState{
		PMastery: cs.PMastery,
		PLearn:   cs.PLearn,
		PForget:  cs.PForget,
		PSlip:    cs.PSlip,
		PGuess:   cs.PGuess,
	}
	bktState = algorithms.BKTUpdate(bktState, success)
	cs.PMastery = bktState.PMastery

	// FSRS ReviewCard.
	rating := algorithms.Good
	if !success {
		rating = algorithms.Again
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

	// IRT UpdateTheta.
	item := algorithms.IRTItem{
		Difficulty:     algorithms.FSRSDifficultyToIRT(cs.Difficulty),
		Discrimination: 1.0,
	}
	cs.Theta = algorithms.IRTUpdateTheta(cs.Theta, []algorithms.IRTItem{item}, []bool{success})

	// PFA Update.
	pfaState := algorithms.PFAState{
		Successes: cs.PFASuccesses,
		Failures:  cs.PFAFailures,
	}
	pfaState = algorithms.PFAUpdate(pfaState, success)
	cs.PFASuccesses = pfaState.Successes
	cs.PFAFailures = pfaState.Failures

	return store.UpsertConceptState(cs)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
