package envmerge

import (
	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// symbolTable is the substitution engine's view of the variable namespace.
type symbolTable = syntax.SymbolTable

// substituter composes variable references over an effective symbol table.
type substituter = syntax.Substituter

// mapSymbols builds a symbolTable over a fully resolved value map keyed the same
// as its origins.
func mapSymbols(
	values map[string]string, origins map[string]Origin,
) syntax.SymbolTable {
	return syntax.SymbolTable{
		Declared: func(name string) bool { _, ok := values[name]; return ok },
		Opaque: func(name string) bool {
			return origins[name].Winner.File == osSource
		},
		Value: func(name string) (string, error) { return values[name], nil },
	}
}

// newSymbolSubstituter builds an engine over an arbitrary symbol table and the
// caller's reference grammar.
func newSymbolSubstituter(
	grammar *syntax.Grammar,
	symbols syntax.SymbolTable,
	getenv func(name string) (string, bool),
	overload bool,
) *syntax.Substituter {
	return syntax.NewSubstituter(syntax.SubstituterParams{
		Grammar:  grammar,
		Symbols:  symbols,
		Getenv:   getenv,
		Overload: overload,
	})
}
