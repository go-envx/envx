package value_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/shared/value"
	"github.com/go-envx/envx/app/internal/utils/severity"
)

func TestKindString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind value.Kind
		want string
	}{
		{value.KindConfig, "config"},
		{value.KindSecret, "secret"},
		{value.KindVariable, "variable"},
		{value.Kind(99), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestKindJSON(t *testing.T) {
	t.Parallel()

	kinds := []value.Kind{value.KindConfig, value.KindSecret, value.KindVariable}
	for _, k := range kinds {
		data, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", k, err)
		}
		var decoded value.Kind
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal(%s): %v", string(data), err)
		}
		if decoded != k {
			t.Errorf("roundtrip = %v, want %v", decoded, k)
		}
	}

	var invalid value.Kind
	err := json.Unmarshal([]byte(`"nonexistent"`), &invalid)
	if err == nil || !errors.Is(err, value.ErrInvalidKind) {
		t.Errorf("expected ErrInvalidKind, got %v", err)
	}
}

func TestValueStructure(t *testing.T) {
	t.Parallel()

	v := value.Value{
		Kind: value.KindVariable,
		Raw:  "{{HOST}}:5432",
		Evaluation: value.Evaluation{
			Severity:      severity.OK,
			Status:        "OK",
			StatusMessage: "ok",
			Value:         "db.local:5432",
			IsResolved:    true,
		},
	}

	if v.Kind != value.KindVariable {
		t.Errorf("Kind = %v, want KindVariable", v.Kind)
	}
	if v.Raw != "{{HOST}}:5432" {
		t.Errorf("Raw = %q, want {{HOST}}:5432", v.Raw)
	}
	if !v.Evaluation.IsResolved {
		t.Error("expected IsResolved to be true")
	}
	if v.Evaluation.Value != "db.local:5432" {
		t.Errorf("Evaluation.Value = %q, want db.local:5432", v.Evaluation.Value)
	}
}

type staticResolver struct{}

func (s staticResolver) Resolve(raw, _ string) (string, error) {
	return "resolved:" + raw, nil
}

type staticEvaluator struct{}

func (s staticEvaluator) Evaluate(raw, _ string) value.Evaluation {
	return value.Evaluation{
		Kind:          value.KindConfig,
		Severity:      severity.OK,
		Status:        "OK",
		StatusMessage: "success",
		Value:         raw,
		IsResolved:    true,
	}
}

func TestInterfaces(t *testing.T) {
	t.Parallel()

	var _ value.Resolver = staticResolver{}
	var _ value.Evaluator = staticEvaluator{}

	r := staticResolver{}
	val, err := r.Resolve("test", "dev")
	if err != nil || val != "resolved:test" {
		t.Errorf("Resolve = (%q, %v), want (resolved:test, nil)", val, err)
	}

	e := staticEvaluator{}
	eval := e.Evaluate("test", "dev")
	if eval.Value != "test" || !eval.IsResolved {
		t.Errorf("Evaluate = %+v, want resolved test", eval)
	}
}
