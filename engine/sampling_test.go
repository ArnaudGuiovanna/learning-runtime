// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package engine

import "testing"

func TestParseEvalResponse_Happy(t *testing.T) {
	got, err := ParseEvalResponse(`{"correct": true, "explanation": "bien vu"}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got.Correct {
		t.Fatalf("Correct: want true")
	}
	if got.Explanation != "bien vu" {
		t.Fatalf("Explanation: %q", got.Explanation)
	}
	if got.ErrorType != "" {
		t.Fatalf("ErrorType: want empty, got %q", got.ErrorType)
	}
}

func TestParseEvalResponse_WithErrorType(t *testing.T) {
	got, err := ParseEvalResponse(`{"correct": false, "explanation": "off by one", "error_type": "off_by_one"}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Correct {
		t.Fatalf("Correct: want false")
	}
	if got.ErrorType != "off_by_one" {
		t.Fatalf("ErrorType: %q", got.ErrorType)
	}
}

func TestParseEvalResponse_StripsCodeFences(t *testing.T) {
	got, err := ParseEvalResponse("```json\n{\"correct\": true, \"explanation\": \"ok\"}\n```")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got.Correct {
		t.Fatalf("Correct: want true")
	}
}

func TestParseEvalResponse_Malformed_Errors(t *testing.T) {
	if _, err := ParseEvalResponse("not json at all"); err == nil {
		t.Fatalf("want err for non-JSON")
	}
}

func TestParseEvalResponse_MissingFields_Errors(t *testing.T) {
	if _, err := ParseEvalResponse(`{"explanation": "no correct field"}`); err == nil {
		t.Fatalf("want err when 'correct' field missing")
	}
}
