package syntax_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
)

// fixedGetenv returns a getenv seam backed by a fixed map so OS lookups are
// hermetic.
func fixedGetenv(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// noGetenv is a getenv seam with no OS variables set.
func noGetenv(string) (string, bool) { return "", false }

// newMapSubstituter is a test helper constructing a Substituter over a map.
func newMapSubstituter(
	table map[string]string,
	getenv func(name string) (string, bool),
	overload bool,
) *syntax.Substituter {
	return syntax.NewSubstituter(syntax.SubstituterParams{
		Symbols:  syntax.MapSymbols(table),
		Getenv:   getenv,
		Overload: overload,
	})
}

// TestResolveLiteralPassthrough verifies a value with no references, and an
// escaped reference, are returned unchanged.
func TestResolveLiteralPassthrough(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"PLAIN":   "postgresql://db:5432",
		"ESCAPED": `\{{HOST}}`,
	}
	s := newMapSubstituter(table, noGetenv, false)

	if got, err := s.Resolve("PLAIN"); err != nil || got != "postgresql://db:5432" {
		t.Errorf("Resolve(PLAIN) = %q, %v; want the literal value", got, err)
	}
	if got, err := s.Resolve("ESCAPED"); err != nil || got != "{{HOST}}" {
		t.Errorf("Resolve(ESCAPED) = %q, %v; want {{HOST}}", got, err)
	}
}

// TestResolveReference verifies a single reference composes the referenced value.
func TestResolveReference(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"HOST": "db.local",
		"URL":  "postgresql://{{HOST}}:5432",
	}
	s := newMapSubstituter(table, noGetenv, false)

	got, err := s.Resolve("URL")
	if err != nil {
		t.Fatalf("Resolve(URL): %v", err)
	}
	if want := "postgresql://db.local:5432"; got != want {
		t.Errorf("Resolve(URL) = %q, want %q", got, want)
	}
}

// TestResolveMultipleReferencesPerValue verifies several references mixed with
// literal text all compose in one value.
func TestResolveMultipleReferencesPerValue(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"SCHEME": "postgresql",
		"HOST":   "db.local",
		"PORT":   "5432",
		"URL":    "{{SCHEME}}://{{HOST}}:{{PORT}}",
	}
	s := newMapSubstituter(table, noGetenv, false)

	got, err := s.Resolve("URL")
	if err != nil {
		t.Fatalf("Resolve(URL): %v", err)
	}
	if want := "postgresql://db.local:5432"; got != want {
		t.Errorf("Resolve(URL) = %q, want %q", got, want)
	}
}

// TestResolveTransitiveChain verifies references compose transitively through a
// chain of variables.
func TestResolveTransitiveChain(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"A": "{{B}}",
		"B": "{{C}}",
		"C": "leaf",
	}
	s := newMapSubstituter(table, noGetenv, false)

	got, err := s.Resolve("A")
	if err != nil {
		t.Fatalf("Resolve(A): %v", err)
	}
	if got != "leaf" {
		t.Errorf("Resolve(A) = %q, want leaf", got)
	}
}

// TestResolveOrderIndependence verifies references resolve regardless of the
// order in which keys are requested or declared.
func TestResolveOrderIndependence(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"FIRST":  "{{SECOND}}-{{THIRD}}",
		"SECOND": "{{THIRD}}",
		"THIRD":  "z",
	}
	want := map[string]string{"FIRST": "z-z", "SECOND": "z", "THIRD": "z"}

	for _, order := range [][]string{
		{"FIRST", "SECOND", "THIRD"},
		{"THIRD", "SECOND", "FIRST"},
		{"SECOND", "FIRST", "THIRD"},
	} {
		s := newMapSubstituter(table, noGetenv, false)
		for _, key := range order {
			got, err := s.Resolve(key)
			if err != nil {
				t.Fatalf("Resolve(%s) order %v: %v", key, order, err)
			}
			if got != want[key] {
				t.Errorf("Resolve(%s) order %v = %q, want %q", key, order, got, want[key])
			}
		}
	}
}

// TestResolveDiamondFanIn verifies a variable referenced through two paths
// composes consistently.
func TestResolveDiamondFanIn(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"TOP":   "{{LEFT}}+{{RIGHT}}",
		"LEFT":  "{{BASE}}",
		"RIGHT": "{{BASE}}",
		"BASE":  "shared",
	}
	s := newMapSubstituter(table, noGetenv, false)

	got, err := s.Resolve("TOP")
	if err != nil {
		t.Fatalf("Resolve(TOP): %v", err)
	}
	if want := "shared+shared"; got != want {
		t.Errorf("Resolve(TOP) = %q, want %q", got, want)
	}
}

// TestResolveReferencePrecedence verifies a reference takes the OS value by
// default and the namespace value under overload.
func TestResolveReferencePrecedence(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"HOST": "ns-host",
		"URL":  "{{HOST}}",
	}
	getenv := fixedGetenv(map[string]string{"HOST": "os-host"})

	if got, err := newMapSubstituter(table, getenv, false).Resolve("URL"); err != nil ||
		got != "os-host" {
		t.Errorf("default Resolve(URL) = %q, %v; want os-host", got, err)
	}
	if got, err := newMapSubstituter(table, getenv, true).Resolve("URL"); err != nil ||
		got != "ns-host" {
		t.Errorf("overload Resolve(URL) = %q, %v; want ns-host", got, err)
	}
}

// TestResolveReferenceFallback verifies a reference falls back to the namespace
// when the OS variable is unset, and to the OS variable under overload when the
// namespace lacks the key.
func TestResolveReferenceFallback(t *testing.T) {
	t.Parallel()

	// Default ordering: OS unset, namespace supplies the value.
	nsTable := map[string]string{"HOST": "ns-host", "URL": "{{HOST}}"}
	if got, err := newMapSubstituter(
		nsTable, noGetenv, false,
	).Resolve("URL"); err != nil || got != "ns-host" {
		t.Errorf("default fallback Resolve(URL) = %q, %v; want ns-host", got, err)
	}

	// Overload ordering: namespace lacks the key, OS supplies the value.
	osTable := map[string]string{"URL": "{{HOST}}"}
	getenv := fixedGetenv(map[string]string{"HOST": "os-host"})
	if got, err := newMapSubstituter(
		osTable, getenv, true,
	).Resolve("URL"); err != nil || got != "os-host" {
		t.Errorf("overload fallback Resolve(URL) = %q, %v; want os-host", got, err)
	}
}

// TestResolveReferenceReentersGraph verifies a reference that resolves to a
// namespace value composes that value's own references.
func TestResolveReferenceReentersGraph(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"URL":    "{{HOST}}",
		"HOST":   "{{REGION}}.db.local",
		"REGION": "eu",
	}
	s := newMapSubstituter(table, noGetenv, false)

	got, err := s.Resolve("URL")
	if err != nil {
		t.Fatalf("Resolve(URL): %v", err)
	}
	if want := "eu.db.local"; got != want {
		t.Errorf("Resolve(URL) = %q, want %q", got, want)
	}
}

// TestResolveMissingReference verifies a reference resolving in neither the
// namespace nor the OS environment is a typed error that names both variables and
// no value.
func TestResolveMissingReference(t *testing.T) {
	t.Parallel()

	table := map[string]string{"URL": "{{HOST}}"}
	_, err := newMapSubstituter(table, noGetenv, false).Resolve("URL")

	var missing *syntax.MissingReferenceError
	if !errors.As(err, &missing) {
		t.Fatalf("Resolve(URL) error = %v, want MissingReferenceError", err)
	}
	if missing.Key != "URL" || missing.Reference != "HOST" {
		t.Errorf(
			"error = {Key:%q Reference:%q}, want {URL HOST}",
			missing.Key, missing.Reference,
		)
	}
}

// TestResolveSelfCycle verifies a length-one cycle is reported with its path.
func TestResolveSelfCycle(t *testing.T) {
	t.Parallel()

	table := map[string]string{"A": "{{A}}"}
	_, err := newMapSubstituter(table, noGetenv, false).Resolve("A")

	var cyclic *syntax.CircularReferenceError
	if !errors.As(err, &cyclic) {
		t.Fatalf("Resolve(A) error = %v, want CircularReferenceError", err)
	}
	if got, want := cyclic.Error(), "circular reference: A -> A"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// TestResolveMultiKeyCycle verifies a longer cycle is reported as an ordered
// path back to its start.
func TestResolveMultiKeyCycle(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"A": "{{B}}",
		"B": "{{C}}",
		"C": "{{A}}",
	}
	_, err := newMapSubstituter(table, noGetenv, false).Resolve("A")

	var cyclic *syntax.CircularReferenceError
	if !errors.As(err, &cyclic) {
		t.Fatalf("Resolve(A) error = %v, want CircularReferenceError", err)
	}
	if got, want := cyclic.Error(), "circular reference: A -> B -> C -> A"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// TestStatusDryRun verifies dry-run classification reports OK, unresolved, and
// circular without exposing a composed value.
func TestStatusDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		table map[string]string
		key   string
		want  syntax.Resolution
	}{
		{
			name:  "resolves",
			table: map[string]string{"A": "{{B}}", "B": "leaf"},
			key:   "A",
			want:  syntax.ResolutionOK,
		},
		{
			name:  "missing reference",
			table: map[string]string{"A": "{{B}}"},
			key:   "A",
			want:  syntax.ResolutionUnresolved,
		},
		{
			name:  "circular reference",
			table: map[string]string{"A": "{{B}}", "B": "{{A}}"},
			key:   "A",
			want:  syntax.ResolutionCircular,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newMapSubstituter(tt.table, noGetenv, false)
			if got := s.Status(tt.key); got != tt.want {
				t.Errorf("Status(%s) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

// TestOpaqueValueNotSubstituted verifies an opaque symbol is never evaluated for
// references.
func TestOpaqueValueNotSubstituted(t *testing.T) {
	t.Parallel()

	s := syntax.NewSubstituter(syntax.SubstituterParams{
		Symbols: syntax.SymbolTable{
			Declared: func(name string) bool { return name == "OPAQUE" },
			Opaque:   func(name string) bool { return name == "OPAQUE" },
			Value:    func(string) (string, error) { return "{{DOES_NOT_EXIST}}", nil },
		},
	})

	got, err := s.Resolve("OPAQUE")
	if err != nil {
		t.Fatalf("Resolve(OPAQUE): %v", err)
	}
	if got != "{{DOES_NOT_EXIST}}" {
		t.Errorf("Resolve(OPAQUE) = %q, want verbatim opaque value", got)
	}
}

// TestResolutionString verifies the String() method on Resolution.
func TestResolutionString(t *testing.T) {
	t.Parallel()

	if got := syntax.ResolutionOK.String(); got != "ok" {
		t.Errorf("ResolutionOK.String() = %q, want ok", got)
	}
	if got := syntax.ResolutionUnresolved.String(); got != "unresolved" {
		t.Errorf("ResolutionUnresolved.String() = %q, want unresolved", got)
	}
	if got := syntax.ResolutionCircular.String(); got != "circular" {
		t.Errorf("ResolutionCircular.String() = %q, want circular", got)
	}
	if got := syntax.Resolution(99).String(); got != "Resolution(99)" {
		t.Errorf("Resolution(99).String() = %q, want Resolution(99)", got)
	}
}
