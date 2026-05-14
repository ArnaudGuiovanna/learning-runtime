// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package engine

import (
	"fmt"
	"sort"
	"strings"

	"tutor-mcp/models"
)

// NudgeCandidate is the scheduler-facing decision made by the nudge planner.
// It separates pedagogical priority from transport details so the scheduler
// can enforce dedup and delivery windows without knowing why a nudge matters.
type NudgeCandidate struct {
	Kind     string
	AlertTag string
	Priority int
	Urgency  models.AlertUrgency
	Brief    models.WebhookBrief
}

// BuildOLMNudgeBrief turns an OLM snapshot into an open-loop Discord brief.
// The message is intentionally incomplete: it tells the learner what is worth
// reopening, but keeps the actual challenge for the tutor session.
func BuildOLMNudgeBrief(snap *OLMSnapshot) models.WebhookBrief {
	if snap == nil {
		return models.WebhookBrief{}
	}
	concept := snap.FocusConcept
	trigger := "Etat d'apprentissage"
	whyNow := "Un point de ton modele d'apprentissage merite une reprise courte aujourd'hui."
	if concept != "" {
		switch snap.FocusUrgency {
		case models.UrgencyCritical:
			trigger = "Rappel prioritaire"
			whyNow = fmt.Sprintf("%s est en zone fragile (%s). Le reprendre maintenant evite une revision plus lourde plus tard.", concept, snap.FocusReason)
		case models.UrgencyWarning:
			trigger = "Focus du moment"
			whyNow = fmt.Sprintf("%s ressort comme le meilleur levier actuel (%s).", concept, snap.FocusReason)
		default:
			trigger = "Prochain palier"
			whyNow = fmt.Sprintf("%s est le prochain petit palier utile.", concept)
		}
	}

	evidence := []string{}
	if buckets := compactBuckets(snap); buckets != "" {
		evidence = append(evidence, "Repartition: "+buckets)
	}
	if snap.FocusReason != "" {
		evidence = append(evidence, "Signal: "+snap.FocusReason)
	}
	if line := MetacogLine(snap); line != "" {
		evidence = append(evidence, "Meta: "+line)
	}

	openLoop := "J'ai garde un micro-defi pour verifier ce point sans te donner la solution ici."
	nextAction := "Ouvre Claude et commence par le focus du jour."
	if concept != "" {
		openLoop = fmt.Sprintf("J'ai garde un micro-defi sur %s: assez court pour le tenter, pas assez trivial pour le faire dans Discord.", concept)
		nextAction = fmt.Sprintf("Ouvre Claude et demande le defi sur %s.", concept)
	}

	return models.WebhookBrief{
		Version:           models.WebhookBriefVersion,
		Kind:              "olm",
		DomainID:          snap.DomainID,
		DomainName:        snap.DomainName,
		Concept:           concept,
		Trigger:           trigger,
		PedagogicalIntent: "orienter la prochaine session vers le meilleur gain d'apprentissage",
		LearningGain:      "Eviter une revision vague: tu sais quoi reprendre, pourquoi, et par quelle action commencer.",
		WhyNow:            whyNow,
		Evidence:          evidence,
		GoalLink:          goalLinkSentence(snap.PersonalGoal, snap.KSTProgress),
		OpenLoop:          openLoop,
		NextAction:        nextAction,
		EstimatedMinutes:  8,
		Language:          "fr",
		Tone:              "clair, concret, encourageant",
	}
}

// BuildMetacognitiveNudgeCandidates converts raw metacognitive alerts into
// learner-facing candidates, sorted by expected learning value. The scheduler
// will enqueue at most one candidate per learner per tick to avoid noisy
// parallel nudges.
func BuildMetacognitiveNudgeCandidates(learner *models.Learner, domains []*models.Domain, alerts []models.Alert) []NudgeCandidate {
	var out []NudgeCandidate
	for _, alert := range alerts {
		kind := metacogKindToWebhookKind(alert.Type)
		if kind == "" {
			continue
		}
		domain := domainForAlert(domains, alert)
		brief := metacognitiveBrief(learner, domain, alert, kind)
		out = append(out, NudgeCandidate{
			Kind:     kind,
			AlertTag: kind,
			Priority: metacognitiveNudgePriority(alert.Type),
			Urgency:  alert.Urgency,
			Brief:    brief,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out
}

func metacognitiveNudgePriority(t models.AlertType) int {
	switch t {
	case models.AlertTransferBlocked:
		return 95
	case models.AlertCalibrationDiverging:
		return 90
	case models.AlertDependencyIncreasing:
		return 80
	case models.AlertAffectNegative:
		return 70
	default:
		return 0
	}
}

func metacognitiveBrief(_ *models.Learner, domain *models.Domain, alert models.Alert, kind string) models.WebhookBrief {
	brief := models.WebhookBrief{
		Version:           models.WebhookBriefVersion,
		Kind:              kind,
		DomainID:          domainID(domain),
		DomainName:        domainName(domain),
		Concept:           alert.Concept,
		EstimatedMinutes:  8,
		Language:          "fr",
		Tone:              "simple, concret, non culpabilisant",
		PedagogicalIntent: "transformer un signal metacognitif en action d'apprentissage courte",
	}

	switch alert.Type {
	case models.AlertTransferBlocked:
		concept := alert.Concept
		if concept == "" {
			concept = "un concept deja travaille"
		}
		brief.Trigger = "Transfert bloque"
		brief.LearningGain = "Transformer une connaissance reconnue en competence reutilisable dans un nouveau contexte."
		brief.WhyNow = fmt.Sprintf("%s semble connu, mais le transfert reste fragile: c'est le bon moment pour changer de contexte.", concept)
		brief.Evidence = []string{"Maitrise elevee detectee", "Au moins deux contextes de transfert faibles"}
		brief.OpenLoop = fmt.Sprintf("J'ai garde une question Feynman sur %s: elle force a expliquer, pas seulement reconnaitre.", concept)
		brief.NextAction = fmt.Sprintf("Ouvre Claude et commence par expliquer %s avec tes mots.", concept)
	case models.AlertCalibrationDiverging:
		brief.Trigger = "Calibration a reprendre"
		brief.LearningGain = "Mieux estimer ton niveau pour choisir des exercices ni trop simples ni trop durs."
		brief.WhyNow = "Ton auto-evaluation s'ecarte de tes resultats recents: une verification a froid donnera une meilleure boussole."
		brief.Evidence = []string{alert.RecommendedAction}
		brief.OpenLoop = "J'ai garde un mini-test sans indice: il dira vite si ton impression colle a ce que tu sais faire."
		brief.NextAction = "Ouvre Claude et commence par une verification courte avant tout nouvel exercice."
	case models.AlertDependencyIncreasing:
		brief.Trigger = "Autonomie en baisse"
		brief.LearningGain = "Reprendre la main progressivement, sans retirer l'aide quand elle est utile."
		brief.WhyNow = "Les derniers signaux montrent plus d'appui sur le tutorat. Le gain vient d'un essai court avec moins d'aide."
		brief.Evidence = []string{"Tendance sur plusieurs sessions", alert.RecommendedAction}
		brief.OpenLoop = "J'ai garde un exercice ou le premier pas est a toi; l'aide revient seulement apres ton essai."
		brief.NextAction = "Ouvre Claude et demande un premier essai sans hint, puis compare avec le feedback."
	case models.AlertAffectNegative:
		brief.Trigger = "Charge a ajuster"
		brief.LearningGain = "Garder la progression sans transformer la difficulte en fatigue inutile."
		brief.WhyNow = "Les dernieres sessions ont ete couteuses. Une reprise plus courte et mieux calibree produira plus d'apprentissage."
		brief.Evidence = []string{"Satisfaction ou difficulte basse sur deux sessions recentes"}
		brief.OpenLoop = "J'ai garde une version allegee du prochain exercice: meme concept, moins de charge parasite."
		brief.NextAction = "Ouvre Claude et commence par une activite courte, puis decide si on augmente."
	default:
		brief.Trigger = "Signal d'apprentissage"
		brief.LearningGain = "Transformer le signal en prochaine action claire."
		brief.WhyNow = alert.RecommendedAction
		brief.OpenLoop = "J'ai garde une question courte pour verifier le point utile."
		brief.NextAction = "Ouvre Claude et commence par ce point."
	}
	brief.GoalLink = goalLinkFromDomain(domain)
	return brief
}

func domainForAlert(domains []*models.Domain, alert models.Alert) *models.Domain {
	if alert.Concept == "" {
		if len(domains) > 0 {
			return domains[0]
		}
		return nil
	}
	for _, d := range domains {
		if d == nil {
			continue
		}
		for _, c := range d.Graph.Concepts {
			if c == alert.Concept {
				return d
			}
		}
	}
	if len(domains) > 0 {
		return domains[0]
	}
	return nil
}

func goalLinkSentence(goal string, progress float64) string {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return ""
	}
	return fmt.Sprintf("Objectif: %s. Etat actuel: %s.", goal, progressPhrase(progress))
}

func goalLinkFromDomain(domain *models.Domain) string {
	if domain == nil || strings.TrimSpace(domain.PersonalGoal) == "" {
		return ""
	}
	return "Lien avec ton objectif: " + strings.TrimSpace(domain.PersonalGoal) + "."
}

func domainID(domain *models.Domain) string {
	if domain == nil {
		return ""
	}
	return domain.ID
}

func domainName(domain *models.Domain) string {
	if domain == nil {
		return ""
	}
	return domain.Name
}
