// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package tools

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterPrompt_ReturnsSystemPrompt(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterPrompt(server)

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "tutor_mcp"})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatalf("expected at least one prompt message")
	}
	msg, ok := res.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent message, got %T", res.Messages[0].Content)
	}
	if !strings.Contains(msg.Text, "tutor MCP") {
		t.Fatalf("expected systemPrompt body to mention 'tutor MCP'")
	}
}

// promptToolBulletRE matches the canonical "OUTILS DISPONIBLES" /
// "OUTILS GOAL-AWARE" entries — a hyphen, a space, a snake_case identifier,
// and an opening parenthesis. This is the strictest lexically-anchored token
// the prompt uses to declare a tool surface and avoids false positives on
// prose that incidentally mentions a tool by name.
var promptToolBulletRE = regexp.MustCompile(`(?m)^- ([a-z_][a-z0-9_]*)\(`)

// extractDocumentedTools pulls the set of tool names declared in the prompt
// header bullet lists. Free-prose mentions inside the rules section are
// intentionally ignored: the bullet list is the contract, the rules are
// commentary.
func extractDocumentedTools(prompt string) map[string]bool {
	out := map[string]bool{}
	for _, m := range promptToolBulletRE.FindAllStringSubmatch(prompt, -1) {
		out[m[1]] = true
	}
	return out
}

// extractRegisteredTools wires up RegisterTools against an in-memory MCP
// server, lists the registered tools and returns their canonical names. This
// mirrors TestRegisterTools_Smoke and ensures the consistency check uses the
// same source of truth as production startup.
func extractRegisteredTools(t *testing.T) map[string]bool {
	t.Helper()
	_, deps := setupToolsTest(t)
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTools(server, deps)

	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	out := map[string]bool{}
	for _, tool := range res.Tools {
		out[tool.Name] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestSystemPrompt_ToolRegistryConsistency asserts that the system prompt
// bullet list and the runtime tool registry stay in lockstep. Drift in
// either direction (registered-but-undocumented, documented-but-unregistered)
// fails the test with a sorted, human-readable list.
//
// Both surfaces are exercised under the same feature-flag configuration:
// REGULATION_GOAL is forced "on" so the goal-aware tools (set_goal_relevance,
// get_goal_relevance) and their corresponding prompt appendix are both
// active and must agree.
func TestSystemPrompt_ToolRegistryConsistency(t *testing.T) {
	// Force a deterministic flag config for the comparison. The action /
	// concept / gate / fade appendices are documentation-only (they don't
	// register new tools), so toggling them does not affect this test —
	// we leave them at their package defaults.
	t.Setenv("REGULATION_GOAL", "on")

	registered := extractRegisteredTools(t)
	prompt := buildSystemPrompt()
	documented := extractDocumentedTools(prompt)

	if len(registered) == 0 {
		t.Fatalf("no registered tools discovered — RegisterTools wiring is broken")
	}
	if len(documented) == 0 {
		t.Fatalf("no documented tools parsed from prompt — bullet format may have changed (regex %q)", promptToolBulletRE.String())
	}

	var registeredButUndocumented []string
	for name := range registered {
		if !documented[name] {
			registeredButUndocumented = append(registeredButUndocumented, name)
		}
	}
	sort.Strings(registeredButUndocumented)

	var documentedButUnregistered []string
	for name := range documented {
		if !registered[name] {
			documentedButUnregistered = append(documentedButUnregistered, name)
		}
	}
	sort.Strings(documentedButUnregistered)

	if len(registeredButUndocumented) > 0 || len(documentedButUnregistered) > 0 {
		t.Fatalf(
			"system prompt vs tool registry drift detected\n"+
				"  registered-but-undocumented (%d): %s\n"+
				"  documented-but-unregistered (%d): %s\n"+
				"  full registered set: %s\n"+
				"  full documented set: %s",
			len(registeredButUndocumented), strings.Join(registeredButUndocumented, ", "),
			len(documentedButUnregistered), strings.Join(documentedButUnregistered, ", "),
			strings.Join(sortedKeys(registered), ", "),
			strings.Join(sortedKeys(documented), ", "),
		)
	}
}

func TestSystemPrompt_RegulationAppendicesAreDefaultOnAndPromptOnly(t *testing.T) {
	t.Setenv("REGULATION_ACTION", "")
	t.Setenv("REGULATION_CONCEPT", "")
	t.Setenv("REGULATION_GATE", "")

	prompt := buildSystemPrompt()

	expected := []string{
		"ACTION-AWARE (REGULATION_ACTION=on):",
		"REGULATION_ACTION is a prompt-only flag.",
		"the runtime action selector still runs through get_next_activity",
		"CONCEPT-AWARE (REGULATION_CONCEPT=on):",
		"REGULATION_CONCEPT is a prompt-only flag.",
		"the runtime concept selector still runs through get_next_activity",
		"GATE-AWARE (REGULATION_GATE=on):",
		"REGULATION_GATE is a prompt-only flag.",
		"the runtime gate still runs through get_next_activity",
	}
	for _, want := range expected {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q", want)
		}
	}
}

func TestSystemPrompt_RegulationAppendixLegacyOffFlags(t *testing.T) {
	t.Setenv("REGULATION_ACTION", "off")
	t.Setenv("REGULATION_CONCEPT", "off")
	t.Setenv("REGULATION_GATE", "off")

	prompt := buildSystemPrompt()

	for _, unwanted := range []string{
		"ACTION-AWARE (REGULATION_ACTION=on):",
		"CONCEPT-AWARE (REGULATION_CONCEPT=on):",
		"GATE-AWARE (REGULATION_GATE=on):",
	} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected legacy flag value off to drop appendix %q", unwanted)
		}
	}
}

func TestSystemPrompt_RegulationAppendixFlagsRequireLiteralOff(t *testing.T) {
	t.Setenv("REGULATION_ACTION", "OFF")
	t.Setenv("REGULATION_CONCEPT", "false")
	t.Setenv("REGULATION_GATE", "0")

	prompt := buildSystemPrompt()

	for _, want := range []string{
		"ACTION-AWARE (REGULATION_ACTION=on):",
		"CONCEPT-AWARE (REGULATION_CONCEPT=on):",
		"GATE-AWARE (REGULATION_GATE=on):",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected non-literal off value to keep appendix %q", want)
		}
	}
}

// TestSystemPrompt_LanguageContract asserts that the system prompt
// instructs the LLM to handle locale per the i18n contract documented
// in docs/i18n.md.
func TestSystemPrompt_LanguageContract(t *testing.T) {
	prompt := buildSystemPrompt()

	if !strings.Contains(prompt, "update_learner_profile") {
		t.Error("system prompt must reference update_learner_profile for language persistence")
	}
	if regexp.MustCompile(`[^\x00-\x7F]`).MatchString(prompt) {
		t.Error("system prompt must be ASCII-only")
	}
	if !strings.Contains(strings.ToLower(prompt), "language") {
		t.Error("system prompt must address language handling explicitly")
	}
	if !strings.Contains(prompt, "profile.language") {
		t.Error("system prompt must describe profile.language as the override")
	}
}
