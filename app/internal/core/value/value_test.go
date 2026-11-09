package value_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

func TestKind_String(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind value.Kind
		want string
	}{
		{value.Config, "config"},
		{value.Secret, "secret"},
		{value.Variable, "variable"},
		{value.Kind(99), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestKind_Constants(t *testing.T) {
	t.Parallel()

	if value.Config != 0 {
		t.Errorf("Config = %d, want 0", value.Config)
	}
	if value.Secret != 1 {
		t.Errorf("Secret = %d, want 1", value.Secret)
	}
	if value.Variable != 2 {
		t.Errorf("Variable = %d, want 2", value.Variable)
	}
}

func TestEvaluation_Structure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		eval value.Evaluation
		want value.Evaluation
	}{
		{
			name: "resolved config",
			eval: value.Evaluation{
				Kind:     value.Config,
				Value:    "localhost:8080",
				Resolved: true,
			},
			want: value.Evaluation{
				Kind:     value.Config,
				Value:    "localhost:8080",
				Resolved: true,
			},
		},
		{
			name: "masked secret",
			eval: value.Evaluation{
				Kind:     value.Secret,
				Value:    "secret://db/password",
				Resolved: false,
			},
			want: value.Evaluation{
				Kind:     value.Secret,
				Value:    "secret://db/password",
				Resolved: false,
			},
		},
		{
			name: "revealed secret",
			eval: value.Evaluation{
				Kind:     value.Secret,
				Value:    "supersecret",
				Resolved: true,
			},
			want: value.Evaluation{
				Kind:     value.Secret,
				Value:    "supersecret",
				Resolved: true,
			},
		},
		{
			name: "unresolved variable with diagnostic",
			eval: value.Evaluation{
				Kind:     value.Variable,
				Value:    "{{MISSING}}",
				Resolved: false,
				Diagnostic: diagnostic.Finding{
					Code:        diagnostic.Code("UNRESOLVED_VARIABLE_REFERENCE"),
					Level:       severity.Error,
					Project:     "api",
					Environment: "dev",
					Key:         "API_KEY",
					Message:     "reference to undefined variable MISSING",
				},
			},
			want: value.Evaluation{
				Kind:     value.Variable,
				Value:    "{{MISSING}}",
				Resolved: false,
				Diagnostic: diagnostic.Finding{
					Code:        diagnostic.Code("UNRESOLVED_VARIABLE_REFERENCE"),
					Level:       severity.Error,
					Project:     "api",
					Environment: "dev",
					Key:         "API_KEY",
					Message:     "reference to undefined variable MISSING",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.eval != tc.want {
				t.Errorf("Evaluation = %+v, want %+v", tc.eval, tc.want)
			}
		})
	}
}

func TestEvaluation_IsZero(t *testing.T) {
	t.Parallel()

	var zero value.Evaluation
	if !zero.IsZero() {
		t.Errorf("zero Evaluation IsZero() = false, want true")
	}

	cases := []struct {
		name string
		eval value.Evaluation
	}{
		{
			name: "non-zero kind",
			eval: value.Evaluation{Kind: value.Secret},
		},
		{
			name: "non-zero value",
			eval: value.Evaluation{Value: "val"},
		},
		{
			name: "non-zero resolved",
			eval: value.Evaluation{Resolved: true},
		},
		{
			name: "non-zero diagnostic",
			eval: value.Evaluation{
				Diagnostic: diagnostic.Finding{
					Code: diagnostic.Code("ERR"),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.eval.IsZero() {
				t.Errorf("%s: IsZero() = true, want false", tc.name)
			}
		})
	}
}
