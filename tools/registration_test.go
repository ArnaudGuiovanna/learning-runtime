// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package tools

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// nonASCII matches any byte outside the 7-bit ASCII range.
var nonASCII = regexp.MustCompile(`[^\x00-\x7F]`)

// TestToolDescriptions_ASCIIOnly walks every non-test Go file in the tools/
// package and asserts that Description: literals and jsonschema:"..." struct
// tags contain only 7-bit ASCII characters.
//
// Rationale: the source language is English (see CONTRIBUTING.md > Language
// and docs/i18n.md). Non-ASCII characters in learner-facing strings indicate
// a leaked non-English string, which the runtime LLM-mediated translation
// contract forbids.
func TestToolDescriptions_ASCIIOnly(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	var failures []string

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.KeyValueExpr:
				key, ok := x.Key.(*ast.Ident)
				if !ok || key.Name != "Description" {
					return true
				}
				lit, ok := x.Value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if nonASCII.MatchString(lit.Value) {
					pos := fset.Position(lit.Pos())
					failures = append(failures, formatFailure(pos.Filename, pos.Line, "non-ASCII in Description", lit.Value))
				}
			case *ast.BasicLit:
				if x.Kind != token.STRING {
					return true
				}
				if !strings.Contains(x.Value, "jsonschema:") {
					return true
				}
				if nonASCII.MatchString(x.Value) {
					pos := fset.Position(x.Pos())
					failures = append(failures, formatFailure(pos.Filename, pos.Line, "non-ASCII in jsonschema tag", x.Value))
				}
			}
			return true
		})
	}

	if len(failures) > 0 {
		t.Fatalf("found %d non-ASCII learner-facing string(s):\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// stringLiteralLintFiles lists files whose every string literal is checked
// against the ASCII-only rule, not just Description / jsonschema tags. Use
// this for handlers known to return learner-facing strings inside map values
// or formatted prompts.
var stringLiteralLintFiles = []string{
	"feynman.go",
}

// TestToolStringLiterals_ASCIIOnly widens the ASCII check to every string
// literal in the files listed by stringLiteralLintFiles.
func TestToolStringLiterals_ASCIIOnly(t *testing.T) {
	fset := token.NewFileSet()
	var failures []string

	for _, path := range stringLiteralLintFiles {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val := lit.Value
			// Skip struct tags handled by the narrower lint above.
			if strings.Contains(val, "jsonschema:") || strings.Contains(val, "json:\"") {
				return true
			}
			if nonASCII.MatchString(val) {
				pos := fset.Position(lit.Pos())
				failures = append(failures, formatFailure(pos.Filename, pos.Line, "non-ASCII in learner-facing string", val))
			}
			return true
		})
	}

	if len(failures) > 0 {
		t.Fatalf("found %d non-ASCII learner-facing string literal(s):\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

func formatFailure(file string, line int, label, val string) string {
	return "  " + file + ":" + itoa(line) + ": " + label + " in " + truncate(val, 120)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
