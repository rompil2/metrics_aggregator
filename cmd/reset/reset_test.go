package main

import (
	"go/ast"
	"go/types"
	"testing"
)

func TestHasResetComment(t *testing.T) {
	doc := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// some comment"},
			{Text: "// generate:reset"},
		},
	}
	if !hasResetComment(doc) {
		t.Error("Expected true for comment with '// generate:reset'")
	}

	doc2 := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// other"},
		},
	}
	if hasResetComment(doc2) {
		t.Error("Expected false for comment without '// generate:reset'")
	}

	if hasResetComment(nil) {
		t.Error("Expected false for nil doc")
	}
}

func TestResetExpressionForType(t *testing.T) {
	tests := []struct {
		typ  types.Type
		name string
		want string
	}{
		{types.Typ[types.Bool], "x", "x = false"},
		{types.Typ[types.String], "x", `x = ""`},
		{types.Typ[types.Int], "x", "x = 0"},
		{types.NewSlice(types.Typ[types.Int]), "x", "x = x[:0]"},
		{types.NewMap(types.Typ[types.String], types.Typ[types.Int]), "x", "clear(x)"},
	}

	for _, tt := range tests {
		got := resetExpressionForType(tt.name, tt.typ)
		if got != tt.want {
			t.Errorf("resetExpressionForType(%v, %q) = %q, want %q", tt.typ, tt.name, got, tt.want)
		}
	}
}

func TestGenerateFieldResetWithTypes(t *testing.T) {
	got := generateFieldResetWithTypes("r", "field", types.Typ[types.String])
	want := `r.field = ""`
	if got != want {
		t.Errorf("generateFieldResetWithTypes = %q, want %q", got, want)
	}
}

func TestHasResetMethod(t *testing.T) {
	// Simple test: a basic type has no Reset method
	if hasResetMethod(types.Typ[types.Int]) {
		t.Error("Expected false for basic type without Reset method")
	}
}
