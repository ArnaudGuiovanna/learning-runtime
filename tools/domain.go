package tools

import (
	"context"
	"fmt"

	"learning-runtime/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InitDomainParams struct {
	Name          string              `json:"name" jsonschema:"Nom du domaine d'apprentissage"`
	Concepts      []string            `json:"concepts" jsonschema:"Liste des concepts du domaine"`
	Prerequisites map[string][]string `json:"prerequisites" jsonschema:"Graphe de prerequis (concept -> liste de prerequis)"`
}

func registerInitDomain(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "init_domain",
		Description: "Initialise un domaine d'apprentissage avec ses concepts et prerequis.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params InitDomainParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		if params.Name == "" {
			r, _ := errorResult("name is required")
			return r, nil, nil
		}
		if len(params.Concepts) == 0 {
			r, _ := errorResult("at least one concept is required")
			return r, nil, nil
		}

		graph := models.KnowledgeSpace{
			Concepts:      params.Concepts,
			Prerequisites: params.Prerequisites,
		}
		if graph.Prerequisites == nil {
			graph.Prerequisites = make(map[string][]string)
		}

		domain, err := deps.Store.CreateDomain(learnerID, params.Name, graph)
		if err != nil {
			r, _ := errorResult(fmt.Sprintf("failed to create domain: %v", err))
			return r, nil, nil
		}

		// Initialize ConceptState for each concept
		for _, concept := range params.Concepts {
			cs := models.NewConceptState(learnerID, concept)
			if err := deps.Store.UpsertConceptState(cs); err != nil {
				r, _ := errorResult(fmt.Sprintf("failed to initialize concept %s: %v", concept, err))
				return r, nil, nil
			}
		}

		r, _ := jsonResult(map[string]interface{}{
			"domain_id":      domain.ID,
			"concept_count":  len(params.Concepts),
			"message":        fmt.Sprintf("Domaine '%s' cree avec %d concepts.", params.Name, len(params.Concepts)),
		})
		return r, nil, nil
	})
}
