// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SetChatModeParams struct {
	Enabled bool `json:"enabled" jsonschema:"Active (true) ou désactive (false) le mode chat-only pour cet apprenant."`
}

func registerSetChatMode(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_chat_mode",
		Description: "Active ou désactive le mode chat-only pour l'apprenant. Quand actif, request_exercise et submit_answer retournent du texte au lieu d'un payload UI iframe — l'expérience chat classique reprend.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, params SetChatModeParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}
		if err := deps.Store.SetChatModeEnabled(learnerID, params.Enabled); err != nil {
			deps.Logger.Error("set_chat_mode: store", "err", err, "learner", learnerID)
			r, _ := errorResult("could not save chat-mode preference")
			return r, nil, nil
		}
		out := map[string]any{"chat_mode_enabled": params.Enabled}
		r, _ := jsonResult(out)
		return r, out, nil
	})
}
