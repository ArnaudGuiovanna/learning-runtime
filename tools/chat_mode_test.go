// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import "testing"

func TestSetChatMode_Toggles(t *testing.T) {
	store, deps := setupToolsTest(t)

	res := callTool(t, deps, registerSetChatMode, "L_owner", "set_chat_mode",
		map[string]any{"enabled": true},
	)
	if res.IsError {
		t.Fatalf("set_chat_mode true errored: %s", resultText(res))
	}
	enabled, _ := store.GetChatModeEnabled("L_owner")
	if !enabled {
		t.Fatalf("expected chat_mode=true after set, got false")
	}

	res = callTool(t, deps, registerSetChatMode, "L_owner", "set_chat_mode",
		map[string]any{"enabled": false},
	)
	if res.IsError {
		t.Fatalf("set_chat_mode false errored: %s", resultText(res))
	}
	enabled, _ = store.GetChatModeEnabled("L_owner")
	if enabled {
		t.Fatalf("expected chat_mode=false after re-set, got true")
	}
}

func TestSetChatMode_NoAuth(t *testing.T) {
	_, deps := setupToolsTest(t)
	res := callTool(t, deps, registerSetChatMode, "", "set_chat_mode",
		map[string]any{"enabled": true},
	)
	if !res.IsError {
		t.Fatalf("expected auth error")
	}
}
