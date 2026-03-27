package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"learning-runtime/auth"
	"learning-runtime/db"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps holds shared dependencies for all MCP tool handlers.
type Deps struct {
	Store *db.Store
}

func getLearnerID(ctx context.Context) (string, error) {
	id := auth.GetLearnerID(ctx)
	if id == "" {
		return "", fmt.Errorf("authentication required")
	}
	return id, nil
}

func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	data, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

func errorResult(msg string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, nil
}

// RegisterTools registers all MCP tools and prompts on the given server.
func RegisterTools(server *mcp.Server, deps *Deps) {
	registerGetPendingAlerts(server, deps)
	registerGetNextActivity(server, deps)
	registerRecordInteraction(server, deps)
	registerCheckMastery(server, deps)
	registerGetLearnerContext(server, deps)
	registerGetAvailabilityModel(server, deps)
	registerGetCockpitState(server, deps)
	registerInitDomain(server, deps)
	RegisterPrompt(server)
}
