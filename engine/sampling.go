// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

// Package engine — sampling helper.
//
// Tutor MCP uses MCP sampling/createMessage to ask the host LLM to generate
// exercise text and feedback evaluations. This file abstracts the SDK call
// behind a small interface so tools can depend on the contract (and test
// against a canned mock) instead of the SDK type.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrSamplingUnsupported is returned by SamplingClient when the host has
// not advertised the sampling capability or the SDK call returns a
// method-not-found / not-supported response. Callers translate this to
// the iframe-side mode:"fallback_b" payload.
var ErrSamplingUnsupported = errors.New("sampling: not supported by host")

// SamplingClient abstracts the MCP host LLM call. Production wiring uses
// the SDK's req.Session.CreateMessage; tests inject a canned implementation.
type SamplingClient interface {
	CreateText(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
}

// EvalResponse is the parsed JSON returned by a sampling call asking the
// LLM to evaluate a learner's answer. submit_answer consumes this shape.
type EvalResponse struct {
	Correct     bool   `json:"correct"`
	Explanation string `json:"explanation"`
	ErrorType   string `json:"error_type,omitempty"`
}

// ParseEvalResponse parses an LLM evaluation response. It is permissive
// about code fences (LLMs frequently wrap JSON in ```json``` despite
// instructions) but strict about the required fields.
func ParseEvalResponse(raw string) (EvalResponse, error) {
	cleaned := stripCodeFences(strings.TrimSpace(raw))
	if !strings.Contains(cleaned, `"correct"`) {
		return EvalResponse{}, fmt.Errorf("eval response missing 'correct' field: %q", cleaned)
	}
	var r EvalResponse
	if err := json.Unmarshal([]byte(cleaned), &r); err != nil {
		return EvalResponse{}, fmt.Errorf("eval response not valid JSON: %w (raw=%q)", err, cleaned)
	}
	return r, nil
}

// fenceRe matches strings wrapped in triple backtick code fences, optionally
// with a language tag. Equivalent to: (?s)^```[a-zA-Z]*\s*(.*?)\s*```$
var fenceRe = regexp.MustCompile("(?s)^`" + "``" + "[a-zA-Z]*\\s*(.*?)\\s*`" + "``" + "$")

func stripCodeFences(s string) string {
	if m := fenceRe.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return s
}

// MCPSamplingClient wires an externally-supplied CreateMessage callback to
// the SamplingClient interface. In production, tools bind this with a
// closure over req.Session.CreateMessage. In tests, callers inject their
// own callback. Stateless: safe to construct per call.
type MCPSamplingClient struct {
	CreateMessage func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
}

func (c MCPSamplingClient) CreateText(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if c.CreateMessage == nil {
		return "", ErrSamplingUnsupported
	}
	return c.CreateMessage(ctx, systemPrompt, userPrompt, maxTokens)
}
