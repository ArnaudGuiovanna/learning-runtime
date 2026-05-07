// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

// Package engine — Open Learner Model graph layer.
//
// OLMGraph extends OLMSnapshot with the full KST graph (nodes + streak) and the
// learner's activity streak. Consumed by the open_app tool's
// structuredContent and by the in-iframe cockpit JS to render the focus card
// and KC list.

package engine

import (
	"fmt"

	"tutor-mcp/algorithms"
	"tutor-mcp/db"
	"tutor-mcp/models"
)

// BuildOLMGraph builds an OLMGraph for the given (learner, domain). If domainID
// is empty, the most recently created non-archived domain is used.
//
// It calls BuildOLMSnapshot for the mastery distribution + focus + metacog +
// KST progress, then enriches with per-concept ConceptState data.
func BuildOLMGraph(store *db.Store, learnerID, domainID string) (*OLMGraph, error) {
	snap, err := BuildOLMSnapshot(store, learnerID, domainID)
	if err != nil {
		return nil, err
	}
	domain, err := resolveActiveDomain(store, learnerID, snap.DomainID)
	if err != nil {
		return nil, fmt.Errorf("olm graph: resolve domain: %w", err)
	}

	// Pull all states once (graph stays under O(N) for the learner).
	allStates, err := store.GetConceptStatesByLearner(learnerID)
	if err != nil {
		return nil, fmt.Errorf("olm graph: get states: %w", err)
	}
	stateByConcept := make(map[string]*models.ConceptState, len(allStates))
	for _, cs := range allStates {
		stateByConcept[cs.Concept] = cs
	}

	// Build nodes — focus state overrides whatever NodeClassify returns.
	nodes := make([]GraphNode, 0, len(domain.Graph.Concepts))
	for _, concept := range domain.Graph.Concepts {
		cs := stateByConcept[concept]
		state := NodeClassify(cs)
		if concept == snap.FocusConcept {
			state = NodeFocus
		}
		n := GraphNode{Concept: concept, State: state}
		if cs != nil {
			n.PMastery = cs.PMastery
			n.Retention = algorithms.Retrievability(cs.ElapsedDays, cs.Stability)
			n.Reps = cs.Reps
			n.Lapses = cs.Lapses
			n.DaysSince = cs.ElapsedDays
		}
		nodes = append(nodes, n)
	}

	// Streak enriches the UI only — DB error means zero, which is safe.
	streak, _ := store.GetActivityStreak(learnerID)

	// Available domains for the cockpit's selector. Best-effort: a DB error
	// here just hides the selector, the rest of the cockpit stays usable.
	var availableDomains []DomainRef
	if domains, err := store.GetDomainsByLearner(learnerID, false /*includeArchived*/); err == nil {
		availableDomains = make([]DomainRef, 0, len(domains))
		for _, d := range domains {
			availableDomains = append(availableDomains, DomainRef{ID: d.ID, Name: d.Name})
		}
	}

	return &OLMGraph{
		OLMSnapshot:      snap,
		Screen:           "cockpit",
		Concepts:         nodes,
		Streak:           streak,
		AvailableDomains: availableDomains,
	}, nil
}

// GraphNode is one concept in the cockpit graph.
type GraphNode struct {
	Concept   string    `json:"concept"`
	State     NodeState `json:"state"`
	PMastery  float64   `json:"p_mastery"`
	Retention float64   `json:"retention"`
	Reps      int       `json:"reps"`
	Lapses    int       `json:"lapses"`
	DaysSince int       `json:"days_since_review"`
}

// DomainRef is one entry in OLMGraph.AvailableDomains — the cockpit's domain
// selector reads this list to let the learner switch between active domains
// without re-running open_cockpit.
type DomainRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OLMGraph is the structured payload exposed to the cockpit iframe.
// It composes OLMSnapshot (mastery distribution + focus + metacog + KST progress)
// with the per-concept node data needed by the V2 cockpit (focus card + KC list).
// Screen is a discriminator field consumed by the iframe SPA dispatcher to
// determine which screen to render (e.g. "cockpit", "exercise", "feedback").
type OLMGraph struct {
	*OLMSnapshot
	Screen           string      `json:"screen"`
	Concepts         []GraphNode `json:"concepts"`
	Streak           int         `json:"streak"`
	AvailableDomains []DomainRef `json:"available_domains"`
}
