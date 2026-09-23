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
		Resolution: value.Resolution{
			Severity:   severity.OK,
			Status:     "OK",
			Value:      "db.local:5432",
			IsResolved: true,
		},
	}

	if v.Kind != value.KindVariable {
		t.Errorf("Kind = %v, want KindVariable", v.Kind)
	}
	if v.Raw != "{{HOST}}:5432" {
		t.Errorf("Raw = %q, want {{HOST}}:5432", v.Raw)
	}
	if !v.Resolution.IsResolved {
		t.Error("expected IsResolved to be true")
	}
	if v.Resolution.Value != "db.local:5432" {
		t.Errorf("Resolution.Value = %q, want db.local:5432", v.Resolution.Value)
	}
}
