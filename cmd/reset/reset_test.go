package main

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
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

func TestFilterLoadedPackages(t *testing.T) {
	// Create a mock package with no errors
	goodPkg := &packages.Package{
		Errors: nil,
		Name:   "goodpkg",
	}

	// Create a mock package with errors
	badPkg := &packages.Package{
		Errors: []packages.Error{},
		Name:   "badpkg",
	}

	input := []*packages.Package{goodPkg, badPkg}
	result := filterLoadedPackages(input)

	assert.Len(t, result, 1)
	assert.Equal(t, "goodpkg", result[0].Name)
}

// Integration test for full workflow
func TestExtractTargetStructsIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	content := `package test

// generate:reset
type MyStruct struct {
	Field int
}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	assert.NoError(t, err)

	// Also create go.mod to make it a proper module
	modContent := `module testmod

go 1.21
`
	modFile := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(modFile, []byte(modContent), 0644)
	assert.NoError(t, err)

	// Load the package
	cfg := &packages.Config{
		Mode:  packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName,
		Tests: false,
		Dir:   tempDir,
	}

	pkgs, err := packages.Load(cfg, ".")
	assert.NoError(t, err)

	// Filter out any packages with errors
	validPkgs := filterLoadedPackages(pkgs)
	if len(validPkgs) == 0 {
		t.Skip("No valid packages found, skipping test")
	}

	// Extract target structs
	allPackages := extractTargetStructs(validPkgs)

	// Verify that the struct was found
	assert.Len(t, allPackages, 1)

	for _, pkgInfo := range allPackages {
		assert.Equal(t, "test", pkgInfo.PackageName)
		assert.Len(t, pkgInfo.Structs, 1)
		assert.Equal(t, "MyStruct", pkgInfo.Structs[0].TypeName)
		assert.Equal(t, "m", pkgInfo.Structs[0].Receiver)
		assert.Len(t, pkgInfo.Structs[0].Fields, 1)
		assert.Contains(t, pkgInfo.Structs[0].Fields[0].Code, "m.Field = 0")
	}
}

func TestGenerateAndWriteFiles(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a mock PackageInfo
	pkgInfo := &PackageInfo{
		PackageName: "test",
		Structs: []StructInfo{
			{
				TypeName: "MyStruct",
				Receiver: "m",
				Fields: []FieldInfo{
					{Code: "m.Field = 0"},
				},
			},
		},
	}

	allPackages := map[string]*PackageInfo{
		tempDir: pkgInfo,
	}

	err := generateAndWriteFiles(allPackages)
	assert.NoError(t, err)

	// Verify that the file was created
	outputPath := filepath.Join(tempDir, "reset.gen.go")
	_, err = os.Stat(outputPath)
	assert.NoError(t, err)

	// Read the file and verify its contents
	data, err := os.ReadFile(outputPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "func (m *MyStruct) Reset()")
	assert.Contains(t, string(data), "m.Field = 0")
}

// Test for empty structs slice
func TestGenerateAndWriteFiles_EmptyStructs(t *testing.T) {
	tempDir := t.TempDir()

	pkgInfo := &PackageInfo{
		PackageName: "test",
		Structs:     []StructInfo{}, // Empty
	}

	allPackages := map[string]*PackageInfo{
		tempDir: pkgInfo,
	}

	err := generateAndWriteFiles(allPackages)
	assert.NoError(t, err)

	// Verify that no file was created
	outputPath := filepath.Join(tempDir, "reset.gen.go")
	_, err = os.Stat(outputPath)
	assert.True(t, os.IsNotExist(err))
}
