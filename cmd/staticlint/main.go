// Package main implements a static analysis tool that combines multiple analyzers.
//
// Usage:
//
//	go run ./cmd/staticlint ./...
//
// The multichecker includes:
//   - All standard analyzers from golang.org/x/tools/go/analysis/passes
//   - All SA-class analyzers from staticcheck (honnef.co/go/tools)
//   - At least one non-SA analyzer from staticcheck (e.g., ST1000)
//   - Two additional public analyzers (e.g., go-critic's ruleguard and unused)
//   - A custom analyzer 'noosexit' that forbids os.Exit in main
//
// See individual analyzer documentation for details.
package main

import (
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	// Staticcheck analyzers
	"honnef.co/go/tools/quickfix"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

// getStaticCheckAnalyzers returns a list of analyzers from the staticcheck suite,
// including SA-class analyzers, at least one non-SA analyzer, and additional public analyzers.
func getStaticCheckAnalyzers() []*analysis.Analyzer {
	var analyzers []*analysis.Analyzer

	// All SA-class analyzers (code smells)
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, "SA") {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// At least one analyzer from other classes
	// S1000 from the simple class
	for _, a := range simple.Analyzers {
		if a.Analyzer.Name == "S1000" {
			analyzers = append(analyzers, a.Analyzer)
			break
		}
	}

	// ST1000 from the stylecheck class
	for _, a := range stylecheck.Analyzers {
		if a.Analyzer.Name == "ST1000" {
			analyzers = append(analyzers, a.Analyzer)
			break
		}
	}

	// QF1000 from the quickfix class
	for _, a := range quickfix.Analyzers {
		if a.Analyzer.Name == "QF1000" {
			analyzers = append(analyzers, a.Analyzer)
			break
		}
	}

	return analyzers
}

func main() {
	var analyzers []*analysis.Analyzer

	// Standard analyzers from x/tools
	standardAnalyzers := []*analysis.Analyzer{
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		ctrlflow.Analyzer,
		deepequalerrors.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		findcall.Analyzer,
		framepointer.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		pkgfact.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	}
	analyzers = append(analyzers, standardAnalyzers...)

	// StaticCheck analyzers, SA and non-SA.
	analyzers = append(analyzers, getStaticCheckAnalyzers()...)

	// Custom analyzer
	analyzers = append(analyzers, NoOsExitAnalyzer)

	multichecker.Main(analyzers...)
}
