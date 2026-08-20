package envmerge

import (
	"errors"
	"testing"
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

// TestTokenize verifies the scanner splits literals from internal and OS
// references, trims whitespace inside the braces, honors the \{{ escape, and
// treats an unterminated opener as literal text.
func TestTokenize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []token
	}{
		{
			name:  "plain literal",
			value: "postgresql://db:5432",
			want:  []token{{tokenLiteral, "postgresql://db:5432"}},
		},
		{
			name:  "internal reference",
			value: "{{HOST}}",
			want:  []token{{tokenInternalRef, "HOST"}},
		},
		{
			name:  "os reference",
			value: "{{@HOST}}",
			want:  []token{{tokenOSRef, "HOST"}},
		},
		{
			name:  "trims whitespace",
			value: "{{  HOST  }}",
			want:  []token{{tokenInternalRef, "HOST"}},
		},
		{
			name:  "trims whitespace after sigil",
			value: "{{@ HOST }}",
			want:  []token{{tokenOSRef, "HOST"}},
		},
		{
			name:  "multiple references and literals",
			value: "{{SCHEME}}://{{@HOST}}:{{PORT}}",
			want: []token{
				{tokenInternalRef, "SCHEME"},
				{tokenLiteral, "://"},
				{tokenOSRef, "HOST"},
				{tokenLiteral, ":"},
				{tokenInternalRef, "PORT"},
			},
		},
		{
			name:  "escaped opener is literal",
			value: `\{{HOST}}`,
			want:  []token{{tokenLiteral, "{{HOST}}"}},
		},
		{
			name:  "escape then live reference",
			value: `\{{LITERAL}} {{HOST}}`,
			want: []token{
				{tokenLiteral, "{{LITERAL}} "},
				{tokenInternalRef, "HOST"},
			},
		},
		{
			name:  "unterminated opener is literal",
			value: "a{{b",
			want:  []token{{tokenLiteral, "a{{b"}},
		},
		{
			name:  "lone backslash is literal",
			value: `a\b`,
			want:  []token{{tokenLiteral, `a\b`}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tokenize(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q) = %v, want %v", tt.value, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf(
						"tokenize(%q)[%d] = %v, want %v",
						tt.value, i, got[i], tt.want[i],
					)
				}
			}
		})
	}
}

// TestResolveLiteralPassthrough verifies a value with no references, and an
// escaped reference, are returned unchanged.
func TestResolveLiteralPassthrough(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"PLAIN":   "postgresql://db:5432",
		"ESCAPED": `\{{HOST}}`,
	}
	s := newSubstituter(table, noGetenv, false)

	if got, err := s.resolve("PLAIN"); err != nil || got != "postgresql://db:5432" {
		t.Errorf("resolve(PLAIN) = %q, %v; want the literal value", got, err)
	}
	if got, err := s.resolve("ESCAPED"); err != nil || got != "{{HOST}}" {
		t.Errorf("resolve(ESCAPED) = %q, %v; want {{HOST}}", got, err)
	}
}

// TestResolveInternalReference verifies a single internal reference composes the
// referenced value.
func TestResolveInternalReference(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"HOST": "db.local",
		"URL":  "postgresql://{{HOST}}:5432",
	}
	s := newSubstituter(table, noGetenv, false)

	got, err := s.resolve("URL")
	if err != nil {
		t.Fatalf("resolve(URL): %v", err)
	}
	if want := "postgresql://db.local:5432"; got != want {
		t.Errorf("resolve(URL) = %q, want %q", got, want)
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
	s := newSubstituter(table, noGetenv, false)

	got, err := s.resolve("URL")
	if err != nil {
		t.Fatalf("resolve(URL): %v", err)
	}
	if want := "postgresql://db.local:5432"; got != want {
		t.Errorf("resolve(URL) = %q, want %q", got, want)
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
	s := newSubstituter(table, noGetenv, false)

	got, err := s.resolve("A")
	if err != nil {
		t.Fatalf("resolve(A): %v", err)
	}
	if got != "leaf" {
		t.Errorf("resolve(A) = %q, want leaf", got)
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

	// Resolving in different orders over independent engines yields the same
	// composed values.
	for _, order := range [][]string{
		{"FIRST", "SECOND", "THIRD"},
		{"THIRD", "SECOND", "FIRST"},
		{"SECOND", "FIRST", "THIRD"},
	} {
		s := newSubstituter(table, noGetenv, false)
		for _, key := range order {
			got, err := s.resolve(key)
			if err != nil {
				t.Fatalf("resolve(%s) order %v: %v", key, order, err)
			}
			if got != want[key] {
				t.Errorf("resolve(%s) order %v = %q, want %q", key, order, got, want[key])
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
	s := newSubstituter(table, noGetenv, false)

	got, err := s.resolve("TOP")
	if err != nil {
		t.Fatalf("resolve(TOP): %v", err)
	}
	if want := "shared+shared"; got != want {
		t.Errorf("resolve(TOP) = %q, want %q", got, want)
	}
}

// TestResolveOSReferencePrecedence verifies {{@VAR}} takes the OS value by
// default and the namespace value under overload.
func TestResolveOSReferencePrecedence(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"HOST": "ns-host",
		"URL":  "{{@HOST}}",
	}
	getenv := fixedGetenv(map[string]string{"HOST": "os-host"})

	if got, err := newSubstituter(table, getenv, false).resolve("URL"); err != nil ||
		got != "os-host" {
		t.Errorf("default resolve(URL) = %q, %v; want os-host", got, err)
	}
	if got, err := newSubstituter(table, getenv, true).resolve("URL"); err != nil ||
		got != "ns-host" {
		t.Errorf("overload resolve(URL) = %q, %v; want ns-host", got, err)
	}
}

// TestResolveOSReferenceFallback verifies {{@VAR}} falls back to the namespace
// when the OS variable is unset, and to the OS variable under overload when the
// namespace lacks the key.
func TestResolveOSReferenceFallback(t *testing.T) {
	t.Parallel()

	// Default ordering: OS unset, namespace supplies the value.
	nsTable := map[string]string{"HOST": "ns-host", "URL": "{{@HOST}}"}
	if got, err := newSubstituter(nsTable, noGetenv, false).resolve("URL"); err != nil ||
		got != "ns-host" {
		t.Errorf("default fallback resolve(URL) = %q, %v; want ns-host", got, err)
	}

	// Overload ordering: namespace lacks the key, OS supplies the value.
	osTable := map[string]string{"URL": "{{@HOST}}"}
	getenv := fixedGetenv(map[string]string{"HOST": "os-host"})
	if got, err := newSubstituter(osTable, getenv, true).resolve("URL"); err != nil ||
		got != "os-host" {
		t.Errorf("overload fallback resolve(URL) = %q, %v; want os-host", got, err)
	}
}

// TestResolveOSReferenceReentersGraph verifies an {{@VAR}} that resolves to a
// namespace value composes that value's own references.
func TestResolveOSReferenceReentersGraph(t *testing.T) {
	t.Parallel()

	table := map[string]string{
		"URL":    "{{@HOST}}",
		"HOST":   "{{REGION}}.db.local",
		"REGION": "eu",
	}
	s := newSubstituter(table, noGetenv, false)

	got, err := s.resolve("URL")
	if err != nil {
		t.Fatalf("resolve(URL): %v", err)
	}
	if want := "eu.db.local"; got != want {
		t.Errorf("resolve(URL) = %q, want %q", got, want)
	}
}

// TestResolveMissingInternalReference verifies a {{VAR}} naming no namespace key
// is a typed error that names both variables and no value.
func TestResolveMissingInternalReference(t *testing.T) {
	t.Parallel()

	table := map[string]string{"URL": "{{HOST}}"}
	_, err := newSubstituter(table, noGetenv, false).resolve("URL")

	var missing *missingInternalReferenceError
	if !errors.As(err, &missing) {
		t.Fatalf("resolve(URL) error = %v, want missingInternalReferenceError", err)
	}
	if missing.key != "URL" || missing.reference != "HOST" {
		t.Errorf(
			"error = {key:%q reference:%q}, want {URL HOST}",
			missing.key, missing.reference,
		)
	}
}

// TestResolveMissingOSReference verifies a {{@VAR}} resolving in neither source
// is a typed error that names both variables and no value.
func TestResolveMissingOSReference(t *testing.T) {
	t.Parallel()

	table := map[string]string{"URL": "{{@HOST}}"}
	_, err := newSubstituter(table, noGetenv, false).resolve("URL")

	var missing *missingOSReferenceError
	if !errors.As(err, &missing) {
		t.Fatalf("resolve(URL) error = %v, want missingOSReferenceError", err)
	}
	if missing.key != "URL" || missing.reference != "HOST" {
		t.Errorf(
			"error = {key:%q reference:%q}, want {URL HOST}",
			missing.key, missing.reference,
		)
	}
}

// TestResolveSelfCycle verifies a length-one cycle is reported with its path.
func TestResolveSelfCycle(t *testing.T) {
	t.Parallel()

	table := map[string]string{"A": "{{A}}"}
	_, err := newSubstituter(table, noGetenv, false).resolve("A")

	var cyclic *circularReferenceError
	if !errors.As(err, &cyclic) {
		t.Fatalf("resolve(A) error = %v, want circularReferenceError", err)
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
	_, err := newSubstituter(table, noGetenv, false).resolve("A")

	var cyclic *circularReferenceError
	if !errors.As(err, &cyclic) {
		t.Fatalf("resolve(A) error = %v, want circularReferenceError", err)
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
		want  substitutionStatus
	}{
		{
			name:  "resolves",
			table: map[string]string{"A": "{{B}}", "B": "leaf"},
			key:   "A",
			want:  statusOK,
		},
		{
			name:  "missing internal reference",
			table: map[string]string{"A": "{{B}}"},
			key:   "A",
			want:  statusUnresolved,
		},
		{
			name:  "missing os reference",
			table: map[string]string{"A": "{{@B}}"},
			key:   "A",
			want:  statusUnresolved,
		},
		{
			name:  "circular reference",
			table: map[string]string{"A": "{{B}}", "B": "{{A}}"},
			key:   "A",
			want:  statusCircular,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newSubstituter(tt.table, noGetenv, false)
			if got := s.status(tt.key); got != tt.want {
				t.Errorf("status(%s) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}
