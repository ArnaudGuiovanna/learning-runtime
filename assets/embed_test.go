// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

package assets

import (
	"strings"
	"testing"
)

func TestEmbeddedCockpitHTML_Present(t *testing.T) {
	data, err := FS.ReadFile("cockpit.html")
	if err != nil {
		t.Fatalf("read cockpit.html: %v", err)
	}
	if len(data) < 50 {
		t.Errorf("cockpit.html too small: %d bytes", len(data))
	}
}

func TestEmbeddedCockpitHTML_HasV2Markers(t *testing.T) {
	data, err := FS.ReadFile("cockpit.html")
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
		// KC picker + j'attaque plumbing — clicks fire MCP tools directly
		// from the iframe (per MCP Apps spec) and j'attaque additionally
		// nudges the LLM via update-model-context.
		"ui/update-model-context",
		"pick_concept",
		"pushModelContext",
		"fireTool",
		"tools/call",
		// JS hooks the click delegation targets
		"data-action=\"attack\"",
		"'data-kc'",
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
			t.Errorf("cockpit.html missing required marker: %q", m)
		}
	}
	if len(data) > 100*1024 {
		t.Errorf("cockpit.html size %d bytes exceeds 100 KB budget", len(data))
	}
}

func TestEmbeddedCockpitHTML_DropsV4Markers(t *testing.T) {
	data, _ := FS.ReadFile("cockpit.html")
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
			t.Errorf("cockpit.html still contains V4 marker that should be removed: %q", m)
		}
	}
}
