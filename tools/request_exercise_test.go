// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"testing"

	"tutor-mcp/models"
)

func TestRequestExercise_HappyPath(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	res := callToolWithSampling(t, deps, registerRequestExercise, "L_owner",
		"request_exercise",
		map[string]any{"domain_id": d.ID},
		"Voici un exercice court sur le concept choisi.",
	)
	if res.IsError {
		t.Fatalf("request_exercise errored: %s", resultText(res))
	}
	out := decodeStructured(t, res)
	if out["screen"] != "exercise" {
		t.Fatalf("expected screen=exercise, got %v", out["screen"])
	}
	ex, ok := out["exercise"].(map[string]any)
	if !ok {
		t.Fatalf("expected exercise object, got %v", out["exercise"])
	}
	if ex["text"] != "Voici un exercice court sur le concept choisi." {
		t.Fatalf("expected sampled text, got %v", ex["text"])
	}
	if _, ok := ex["concept"]; !ok {
		t.Fatalf("expected concept field in exercise")
	}
	if _, ok := ex["activity_type"]; !ok {
		t.Fatalf("expected activity_type field in exercise")
	}
}

func TestRequestExercise_SamplingUnsupported_FallbackB(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	// Empty samplingResponse → no CreateMessageHandler on test client →
	// SDK returns method-not-found, server-side surfaces fallback.
	res := callToolWithSampling(t, deps, registerRequestExercise, "L_owner",
		"request_exercise",
		map[string]any{"domain_id": d.ID},
		"", // unsupported
	)
	if res.IsError {
		t.Fatalf("request_exercise errored: %s", resultText(res))
	}
	out := decodeStructured(t, res)
	if out["mode"] != "fallback_b" {
		t.Fatalf("expected mode=fallback_b, got %v", out["mode"])
	}
	ex := out["exercise"].(map[string]any)
	if ex["prompt_for_llm"] == nil || ex["prompt_for_llm"] == "" {
		t.Fatalf("expected prompt_for_llm in fallback exercise, got %v", ex)
	}
	if _, hasText := ex["text"]; hasText {
		t.Fatalf("did not expect text field in fallback, got %v", ex["text"])
	}
}

func TestRequestExercise_ChatMode_TextOnly(t *testing.T) {
	store, deps := setupToolsTest(t)
	d := makeOwnerDomain(t, store, "L_owner", "math")
	cs := models.NewConceptState("L_owner", "a")
	cs.PMastery = 0.30
	_ = store.InsertConceptStateIfNotExists(cs)
	_ = store.UpsertConceptState(cs)

	if err := store.SetChatModeEnabled("L_owner", true); err != nil {
		t.Fatalf("SetChatModeEnabled: %v", err)
	}

	res := callToolWithSampling(t, deps, registerRequestExercise, "L_owner",
		"request_exercise",
		map[string]any{"domain_id": d.ID},
		"Voici un exercice.",
	)
	if res.IsError {
		t.Fatalf("request_exercise errored: %s", resultText(res))
	}
	// In chat mode, structuredContent must NOT be set — the host should
	// not render the iframe; the LLM speaks the exercise in chat.
	if res.StructuredContent != nil {
		t.Fatalf("expected nil StructuredContent in chat mode, got %v", res.StructuredContent)
	}
	out := decodeResult(t, res) // text-content JSON
	if out["chat_mode"] != true {
		t.Fatalf("expected chat_mode=true in text payload, got %v", out["chat_mode"])
	}
}
