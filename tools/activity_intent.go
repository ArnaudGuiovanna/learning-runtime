// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"tutor-mcp/algorithms"
	"tutor-mcp/db"
	"tutor-mcp/models"
)

const (
	activityIntentAuto   = "auto"
	activityIntentReview = "review"
)

func normalizeActivityIntent(raw string) (string, error) {
	token := compactToken(raw)
	switch token {
	case "", "auto":
		return activityIntentAuto, nil
	case "review", "revise", "revision", "reviser", "practice", "practise":
		return activityIntentReview, nil
	default:
		return "", fmt.Errorf("unsupported intent %q: allowed values are auto, review", raw)
	}
}

func resolveActivityDomain(store *db.Store, learnerID, domainID, domainName string) (*models.Domain, error) {
	if domainID != "" || strings.TrimSpace(domainName) == "" {
		return resolveDomain(store, learnerID, domainID)
	}

	domains, err := store.GetDomainsByLearner(learnerID, false)
	if err != nil {
		return nil, err
	}
	needle := compactToken(domainName)
	if needle == "" {
		return nil, fmt.Errorf("domain_name is empty after normalization")
	}

	for _, d := range domains {
		if compactToken(d.Name) == needle {
			return d, nil
		}
	}

	var matches []*models.Domain
	for _, d := range domains {
		haystack := compactToken(d.Name)
		if strings.Contains(haystack, needle) || strings.Contains(needle, haystack) {
			matches = append(matches, d)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("domain_name %q did not match an active domain; available domains: %s", domainName, domainNames(domains))
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("domain_name %q is ambiguous; matching domains: %s", domainName, domainNames(matches))
	}
}

func compactToken(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		r = unicode.ToLower(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func domainNames(domains []*models.Domain) string {
	if len(domains) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(domains))
	for _, d := range domains {
		parts = append(parts, fmt.Sprintf("%s (%s)", d.Name, d.ID))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

type reviewCandidate struct {
	concept      string
	state        *models.ConceptState
	score        float64
	retention    float64
	hasActiveMis bool
}

func selectReviewIntentActivity(store *db.Store, learnerID string, domain *models.Domain, states []*models.ConceptState, interactions []*models.Interaction, sessionConcepts map[string]int, now time.Time) (models.Activity, string) {
	candidates := reviewCandidatesForDomain(store, learnerID, domain, states, interactions, sessionConcepts, now, true)
	if len(candidates) == 0 {
		candidates = reviewCandidatesForDomain(store, learnerID, domain, states, interactions, sessionConcepts, now, false)
	}
	if len(candidates) == 0 {
		return models.Activity{
			Type:             models.ActivityRest,
			DifficultyTarget: 0.3,
			Format:           "review_unavailable",
			EstimatedMinutes: 3,
			Rationale:        "[intent=review] no previously studied concept is available in this domain",
			PromptForLLM:     "No reviewed concept is available in this domain. Tell the learner there is nothing to revise yet in this domain and ask whether they want to start a new concept.",
		}, "no_reviewable_concept"
	}

	c := candidates[0]
	activityType := models.ActivityPractice
	format := "review_practice"
	difficulty := 0.55
	minutes := 10
	if c.hasActiveMis {
		activityType = models.ActivityDebugMisconception
		format = "targeted_review"
		difficulty = 0.6
		minutes = 12
	} else if c.state != nil && c.state.CardState != "" && c.state.CardState != "new" {
		activityType = models.ActivityRecall
		format = "retrieval_review"
		difficulty = 0.6
		minutes = 8
	}

	return models.Activity{
		Type:             activityType,
		Concept:          c.concept,
		DifficultyTarget: difficulty,
		Format:           format,
		EstimatedMinutes: minutes,
		Rationale:        fmt.Sprintf("[intent=review] selected prior concept %q with retention %.2f", c.concept, c.retention),
		PromptForLLM:     fmt.Sprintf("Generate a review activity on %s. Do not introduce a new concept; focus on retrieval or applied practice from prior material.", c.concept),
	}, "applied"
}

func reviewCandidatesForDomain(store *db.Store, learnerID string, domain *models.Domain, states []*models.ConceptState, interactions []*models.Interaction, sessionConcepts map[string]int, now time.Time, skipSessionConcepts bool) []reviewCandidate {
	stateByConcept := map[string]*models.ConceptState{}
	for _, cs := range states {
		stateByConcept[cs.Concept] = cs
	}
	seenByInteraction := map[string]bool{}
	for _, i := range interactions {
		seenByInteraction[i.Concept] = true
	}

	candidates := make([]reviewCandidate, 0, len(domain.Graph.Concepts))
	for _, concept := range domain.Graph.Concepts {
		if skipSessionConcepts && sessionConcepts[concept] > 0 {
			continue
		}
		cs := stateByConcept[concept]
		if !isReviewableConcept(cs, seenByInteraction[concept]) {
			continue
		}

		retention := 1.0
		score := 0.1
		if cs != nil {
			retention = reviewRetention(cs, now)
			score += (1 - retention) * 2
			score += (1 - clampUnit(cs.PMastery)) * 0.5
			if cs.Lapses > 0 {
				score += 0.2
			}
		}
		hasMis := false
		if mis, err := store.GetFirstActiveMisconception(learnerID, concept); err == nil && mis != nil {
			hasMis = true
			score += 1.0
		}

		candidates = append(candidates, reviewCandidate{
			concept:      concept,
			state:        cs,
			score:        score,
			retention:    retention,
			hasActiveMis: hasMis,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].concept < candidates[j].concept
		}
		return candidates[i].score > candidates[j].score
	})
	return candidates
}

func isReviewableConcept(cs *models.ConceptState, seenInteraction bool) bool {
	if seenInteraction {
		return true
	}
	if cs == nil {
		return false
	}
	return cs.Reps > 0 || cs.LastReview != nil || (cs.CardState != "" && cs.CardState != "new")
}

func reviewRetention(cs *models.ConceptState, now time.Time) float64 {
	if cs == nil || cs.Stability <= 0 || cs.CardState == "" || cs.CardState == "new" {
		return 1.0
	}
	elapsedDays := cs.ElapsedDays
	if cs.LastReview != nil {
		elapsedDays = int(now.Sub(*cs.LastReview).Hours() / 24)
		if elapsedDays < 0 {
			elapsedDays = 0
		}
	}
	return clampUnit(algorithms.Retrievability(elapsedDays, cs.Stability))
}
