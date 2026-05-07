// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"encoding/json"

	"tutor-mcp/assets"
	"tutor-mcp/auth"
	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// OpenAppParams is the input shape for open_app and its legacy alias open_cockpit.
type OpenAppParams struct {
	DomainID string `json:"domain_id,omitempty" jsonschema:"ID du domaine (optionnel, dernier domaine actif si absent)"`
}

// OpenCockpitParams is the legacy name for OpenAppParams kept as an alias
// so existing callers compile unchanged.
type OpenCockpitParams = OpenAppParams

// appResourceURI is the MCP Apps resource URI for the app UI.
// This is the renamed cockpit.html → app.html in Task 7; for now the
// resource still serves cockpit.html under the new URI.
const appResourceURI = "ui://app"

// appUIMeta returns a fresh _meta payload pointing at the app resource —
// used both on the Tool.Meta (so clients see the resource URI before calling)
// and on CallToolResult.Meta (so the client knows which resource to fetch
// after calling).
func appUIMeta() mcp.Meta {
	return mcp.Meta{
		"ui": map[string]any{
			"resourceUri": appResourceURI,
			"visibility":  []string{"model", "app"},
		},
	}
}

// registerOpenApp registers the open_app tool — entry point for the
// MCP App iframe. Returns the OLMGraph as structuredContent with
// screen:"cockpit". The legacy open_cockpit is kept as a thin alias
// (tools/cockpit.go) using the same shared handler.
func registerOpenApp(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_app",
		Description: "Ouvre l'app Tutor MCP (interface complète : cockpit, exercices, feedback). Utiliser quand l'apprenant demande d'ouvrir/voir/afficher son tutor ou son cockpit. Rend une UI MCP App native dans la conversation. NE PAS reformuler le résultat en texte : la UI s'affiche d'elle-même via _meta.ui.resourceUri. Pour les clients sans MCP Apps, le tool retourne aussi un résumé texte de fallback.",
		Meta:        appUIMeta(),
	}, openAppHandler(deps))
}

// openAppHandler returns the closure shared between registerOpenApp and
// registerOpenCockpit (alias). The body is the existing open_cockpit
// implementation, moved here unchanged.
func openAppHandler(deps *Deps) func(context.Context, *mcp.CallToolRequest, OpenCockpitParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, params OpenCockpitParams) (*mcp.CallToolResult, any, error) {
		learnerID, err := getLearnerID(ctx)
		if err != nil {
			deps.Logger.Error("open_app: auth failed", "err", err)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		graph, err := engine.BuildOLMGraph(deps.Store, learnerID, params.DomainID)
		if err != nil {
			deps.Logger.Error("open_app: build graph failed", "err", err, "learner", learnerID)
			r, _ := errorResult(err.Error())
			return r, nil, nil
		}

		// Embed a short-lived JWT and the API base URL so the iframe can make
		// direct fetch() calls to /api/v1/* without going through the MCP App
		// protocol (which 405s on claude.ai web for tools/call and drops
		// ui/message silently).
		sessionToken, jwtErr := auth.GenerateJWT(deps.BaseURL, learnerID)
		if jwtErr != nil {
			deps.Logger.Error("open_app: jwt issue", "err", jwtErr)
			// Continue without — iframe will show "auth missing" and fall
			// back gracefully.
		}

		// Merge token + api_base into the graph payload as extra top-level
		// fields. Marshal the OLMGraph to a map, inject, then pass to
		// structuredContent.
		out := map[string]any{}
		if b, err2 := json.Marshal(graph); err2 == nil {
			_ = json.Unmarshal(b, &out)
		}
		if sessionToken != "" {
			out["_session_token"] = sessionToken
		}
		out["_api_base"] = deps.BaseURL

		// Text fallback for clients without MCP Apps support — reuses the
		// webhook formatter so cockpit and webhook show the same prose.
		fallback := engine.FormatOLMEmbed(graph.OLMSnapshot)

		r, _ := jsonResult(out)
		r.Content = []mcp.Content{&mcp.TextContent{Text: fallback.Description}}
		r.Meta = appUIMeta()
		return r, nil, nil
	}
}

// registerAppResource serves the iframe HTML at ui://app. The same HTML
// is also served at ui://cockpit (legacy alias) by registerCockpitResource
// for backward compat.
func registerAppResource(server *mcp.Server, deps *Deps) {
	server.AddResource(&mcp.Resource{
		URI:         appResourceURI,
		Name:        "app",
		Title:       "Tutor MCP App",
		Description: "App UI rendered as an MCP App iframe.",
		MIMEType:    "text/html;profile=mcp-app",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := ""
		if req != nil && req.Params != nil {
			uri = req.Params.URI
		}
		deps.Logger.Info("app resource read", "uri", uri)
		body, err := assets.FS.ReadFile("app.html")
		if err != nil {
			deps.Logger.Error("app resource: read embedded html", "err", err)
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      appResourceURI,
				MIMEType: "text/html;profile=mcp-app",
				Text:     string(body),
				// connectDomains whitelists our server origin so the iframe
				// sandbox CSP allows fetch() to the /api/v1/* endpoints.
				// Per MCP Apps spec 2026-01-26 §CSP, connectDomains is the
				// canonical mechanism for this.
				Meta: mcp.Meta{
					"ui": map[string]any{
						"csp": map[string]any{
							"connectDomains": []string{deps.BaseURL},
						},
					},
				},
			}},
		}, nil
	})
}
