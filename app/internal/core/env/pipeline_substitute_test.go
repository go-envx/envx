package env

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/env/syntax"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// substituteOp is an operation with the default grammar and the given host.
func substituteOp(host hostenv.Snapshot, overload bool) operation {
	return operation{
		grammar:  &syntax.Grammar{},
		host:     host,
		settings: resolvedSettings{overload: overload},
	}
}

// tableSubstituter builds an engine over a plain table with no opaque values.
func tableSubstituter(
	table map[string]string, host hostenv.Snapshot, overload bool,
) *substituter {
	return newSubstituter(substituteOp(host, overload), symbolTable{
		declared: func(name string) bool { _, ok := table[name]; return ok },
		opaque:   func(string) bool { return false },
		value:    func(name string) (string, error) { return table[name], nil },
	})
}

// TestSubstituterResolve verifies composition through chains, fan-in, and
// escapes regardless of request order.
func TestSubstituterResolve(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"SCHEME":  "postgresql",
		"REGION":  "eu",
		"HOST":    "{{REGION}}.db.local",
		"URL":     "{{SCHEME}}://{{HOST}}:5432",
		"TOP":     "{{LEFT}}+{{RIGHT}}",
		"LEFT":    "{{REGION}}",
		"RIGHT":   "{{REGION}}",
		"ESCAPED": `\{{HOST}}`,
	}
	want := map[string]string{
		"URL":     "postgresql://eu.db.local:5432",
		"TOP":     "eu+eu",
		"ESCAPED": "{{HOST}}",
		"HOST":    "eu.db.local",
	}
	for _, order := range [][]string{
		{"URL", "TOP", "ESCAPED", "HOST"},
		{"HOST", "ESCAPED", "TOP", "URL"},
	} {
		engine := tableSubstituter(table, hostenv.Snapshot{}, false)
		for _, key := range order {
			if got, err := engine.resolve(key); err != nil || got != want[key] {
				t.Errorf("resolve(%s) = %q, %v; want %q", key, got, err, want[key])
			}
		}
	}
}

// TestSubstituterPrecedence verifies host values win by default and namespace
// values win under overload, each falling back to the other.
func TestSubstituterPrecedence(t *testing.T) {
	t.Parallel()

	host := hostenv.From([]string{"HOST=host", "ONLY_HOST=h"})
	table := map[string]string{
		"HOST": "ns", "ONLY_NS": "n", "URL": "{{HOST}}/{{ONLY_HOST}}/{{ONLY_NS}}",
	}

	if got, _ := tableSubstituter(table, host, false).resolve("URL"); got != "host/h/n" {
		t.Errorf("default resolve = %q, want host/h/n", got)
	}
	if got, _ := tableSubstituter(table, host, true).resolve("URL"); got != "ns/h/n" {
		t.Errorf("overload resolve = %q, want ns/h/n", got)
	}
}

// TestSubstituterSupersededFailure verifies a namespace value the host
// supersedes is never resolved.
func TestSubstituterSupersededFailure(t *testing.T) {
	t.Parallel()

	op := substituteOp(hostenv.From([]string{"BROKEN=ok"}), false)
	engine := newSubstituter(op, symbolTable{
		declared: func(string) bool { return true },
		opaque:   func(string) bool { return false },
		value: func(name string) (string, error) {
			if name == "BROKEN" {
				return "", errors.New("broken")
			}
			return "{{BROKEN}}", nil
		},
	})
	if got, err := engine.resolve("URL"); err != nil || got != "ok" {
		t.Errorf("resolve = %q, %v; want ok", got, err)
	}
}

// TestSubstituterErrors verifies missing references and cycles are typed
// errors naming only variables.
func TestSubstituterErrors(t *testing.T) {
	t.Parallel()

	_, err := tableSubstituter(
		map[string]string{"URL": "{{HOST}}"}, hostenv.Snapshot{}, false,
	).resolve("URL")
	var missing *MissingReferenceError
	if !errors.As(err, &missing) || missing.Key != "URL" || missing.Reference != "HOST" {
		t.Errorf("missing error = %v", err)
	}

	tests := map[string]map[string]string{
		"circular reference: A -> A":           {"A": "{{A}}"},
		"circular reference: A -> B -> C -> A": {"A": "{{B}}", "B": "{{C}}", "C": "{{A}}"},
	}
	for want, table := range tests {
		_, err := tableSubstituter(table, hostenv.Snapshot{}, false).resolve("A")
		if !errors.Is(err, ErrCircularReference) || err.Error() != want {
			t.Errorf("cycle error = %v, want %q", err, want)
		}
	}
}

// TestSubstituterOpaque verifies opaque values are returned verbatim.
func TestSubstituterOpaque(t *testing.T) {
	t.Parallel()

	engine := newSubstituter(substituteOp(hostenv.Snapshot{}, false), symbolTable{
		declared: func(string) bool { return true },
		opaque:   func(string) bool { return true },
		value:    func(string) (string, error) { return "{{MISSING}}", nil },
	})
	if got, err := engine.resolve("OPAQUE"); err != nil || got != "{{MISSING}}" {
		t.Errorf("resolve = %q, %v; want verbatim", got, err)
	}
}

// TestSubstituterMemoizes verifies each symbol value is read once.
func TestSubstituterMemoizes(t *testing.T) {
	t.Parallel()

	reads := map[string]int{}
	table := map[string]string{"A": "{{C}}{{C}}", "B": "{{C}}", "C": "x"}
	engine := newSubstituter(substituteOp(hostenv.Snapshot{}, false), symbolTable{
		declared: func(name string) bool { _, ok := table[name]; return ok },
		opaque:   func(string) bool { return false },
		value: func(name string) (string, error) {
			reads[name]++
			return table[name], nil
		},
	})
	for _, key := range []string{"A", "B", "A"} {
		if _, err := engine.resolve(key); err != nil {
			t.Fatalf("resolve(%s): %v", key, err)
		}
	}
	if reads["C"] != 1 || reads["A"] != 1 {
		t.Errorf("reads = %v, want one per symbol", reads)
	}
}
