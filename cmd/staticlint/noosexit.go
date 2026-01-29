// Package noosexit defines an analyzer that forbids direct calls to os.Exit in the main function of the main package.
package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NoOsExitAnalyzer is an analyzer that reports diagnostics if os.Exit is called directly inside the main function
// of the main package. It enforces graceful shutdown patterns by discouraging direct process termination.
var NoOsExitAnalyzer = &analysis.Analyzer{
	Name:     "noosexit",
	Doc:      "forbids direct os.Exit calls in main function of main package",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// run executes the logic of the NoOsExitAnalyzer.
// It inspects the AST of the current package and identifies direct calls to os.Exit within the main function.
// Reports a diagnostic if such a call is found, suggesting alternative approaches.
func run(pass *analysis.Pass) (interface{}, error) {
	// Check if current package is "main"
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" {
			return
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "os" {
				return true
			}

			if sel.Sel.Name == "Exit" {
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: "direct call to os.Exit in main is forbidden",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Consider returning an error or using a different exit strategy",
						},
					},
				})
			}
			return true
		})
	})

	return nil, nil
}
