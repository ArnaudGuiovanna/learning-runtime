// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"

	"tutor-mcp/apihttp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SubmitExerciseContentParams is the schema the host LLM fills in when
// the iframe asked for exercise content via ui/message. The iframe
// includes the request_id in the chat payload; the LLM echoes it back
// here alongside the generated énoncé.
type SubmitExerciseContentParams struct {
	RequestID string `json:"request_id" jsonschema:"required,L'identifiant que l'iframe a fourni dans son ui/message (champ request_id)"`
	Content   string `json:"content" jsonschema:"required,L'énoncé de l'exercice généré par toi (Claude). Markdown léger autorisé. Pas de solution, pas de hints inline."`
}

func registerSubmitExerciseContent(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "submit_exercise_content",
		Description: "Appelle cet outil quand l'iframe Tutor t'envoie un message commençant par GENERATE_EXERCISE. Tu reçois un request_id et une description (concept + type d'activité + difficulté). Génère un énoncé pédagogique adapté et retourne-le via cet outil. NE PAS afficher l'énoncé dans le chat — l'iframe l'affichera elle-même. NE PAS produire de solution ni de hints. Style clair, concis. Markdown léger autorisé.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params SubmitExerciseContentParams) (*mcp.CallToolResult, any, error) {
		if params.RequestID == "" || params.Content == "" {
			r, _ := errorResult("request_id and content required")
			return r, nil, nil
		}
		apihttp.PutContent(params.RequestID, params.Content)
		deps.Logger.Info("submit_exercise_content: stored", "id", params.RequestID, "chars", len(params.Content))
		// Tiny ack — the LLM doesn't need to relay this to the user.
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil, nil
	})
}
