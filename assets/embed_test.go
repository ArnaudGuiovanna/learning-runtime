// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package assets

import (
	"strings"
	"testing"
)

func TestEmbeddedCockpitHTML_Present(t *testing.T) {
	data, err := FS.ReadFile("app.html")
	if err != nil {
		t.Fatalf("read app.html: %v", err)
	}
	if len(data) < 50 {
		t.Errorf("app.html too small: %d bytes", len(data))
	}
}

func TestEmbeddedCockpitHTML_HasV2Markers(t *testing.T) {
	data, err := FS.ReadFile("app.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	mustContain := []string{
		// V2 visual tokens
		"--ck-bg-from:",
		"--ck-bg-to:",
		"--ck-accent-500: #e8804a",
		"--ck-text-primary:",
		// V2 class hooks
		".ck-frame",
		".ck-focus",
		".ck-kc-row",
		".ck-stats",
		".ck-signal",
		".ck-toast-region",
		// Responsive (mobile-first)
		"container-type: inline-size",
		"@container (max-width: 480px)",
		"prefers-reduced-motion",
		// MCP App handshake
		"ui/initialize",
		"ui/notifications/initialized",
		"ui/notifications/tool-result",
		"ui/notifications/size-changed",
		"ui/notifications/host-context-changed",
		"ui/request-display-mode",
		"availableDisplayModes",
		"\"fullscreen\"",
		// KC picker + j'attaque plumbing — clicks go via sendChatMessage
		// (ui/message) to trigger an immediate LLM turn per MCP Apps spec.
		// pushModelContext/ui/update-model-context remain as fallback path.
		"ui/update-model-context",
		"pick_concept",
		"pushModelContext",
		"fireTool",
		"tools/call",
		"sendChatMessage",
		"ui/message",
		"data-action=\"expand-kcs\"",
		"ck-kc-more",
		// V3 dispatch + 3 renderers
		"\"screen\"",
		"renderCockpit",
		"renderExercise",
		"renderFeedback",
		"dispatchScreen",
		"RENDERERS",
		"request_exercise",
		"submit_answer",
		// Direct HTTP API (replaces MCP App protocol for click actions)
		"apiCall",
		"_session_token",
		"_api_base",
		"/api/v1/exercise",
		"/api/v1/cockpit",
		"/api/v1/submit",
		"/api/v1/pick_concept",
		// Click delegation targets
		"data-action=\"attack\"",
		"data-action=\"submit\"",
		"data-action=\"continue\"",
		"'data-kc'",
		// 10-second re-enable timeout wiring (disabled-button regression fix)
		"_attackTimeout",
		"_submitTimeout",
		"_continueTimeout",
		// DOM ids the JS targets
		"id=\"ck-domain-select\"",
		"id=\"ck-fullscreen-btn\"",
		"id=\"ck-toast-region\"",
		"id=\"ck-focus-card\"",
		"id=\"ck-kc-list\"",
		"id=\"ck-stats\"",
		"id=\"ck-signal\"",
	}
	for _, m := range mustContain {
		if !strings.Contains(body, m) {
			t.Errorf("app.html missing required marker: %q", m)
		}
	}
	if len(data) > 100*1024 {
		t.Errorf("app.html size %d bytes exceeds 100 KB budget", len(data))
	}
}

func TestEmbeddedCockpitHTML_DropsV4Markers(t *testing.T) {
	data, _ := FS.ReadFile("app.html")
	body := string(data)
	mustNotContain := []string{
		// V4 graph (carte cognitive) — dropped
		"id=\"olm-graph\"",
		"cv4-pulse-halo",
		"cv4-fade-up",
		"renderGraph",
		// V4 tabs — dropped (single-scroll now)
		"id=\"tab-global\"",
		"id=\"panel-global\"",
		"renderGlobal",
		"role=\"tablist\"",
	}
	for _, m := range mustNotContain {
		if strings.Contains(body, m) {
			t.Errorf("app.html still contains V4 marker that should be removed: %q", m)
		}
	}
}
