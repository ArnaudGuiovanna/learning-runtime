// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"fmt"

	"tutor-mcp/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// callSampling wraps req.Session.CreateMessage. Returns engine.ErrSamplingUnsupported
// when the host has not advertised sampling or the SDK call returns an error.
func callSampling(ctx context.Context, req *mcp.CallToolRequest, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	resp, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		MaxTokens:    int64(maxTokens),
		SystemPrompt: systemPrompt,
		Messages: []*mcp.SamplingMessage{
			{Role: "user", Content: &mcp.TextContent{Text: userPrompt}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", engine.ErrSamplingUnsupported, err)
	}
	if resp == nil || resp.Content == nil {
		return "", engine.ErrSamplingUnsupported
	}
	if tc, ok := resp.Content.(*mcp.TextContent); ok {
		return tc.Text, nil
	}
	return "", engine.ErrSamplingUnsupported
}
