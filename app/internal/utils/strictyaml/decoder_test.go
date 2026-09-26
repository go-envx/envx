package strictyaml_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/utils/strictyaml"
)

type sampleRoot struct {
	Name     string         `yaml:"name"`
	Settings sampleSettings `yaml:"settings"`
}

type sampleSettings struct {
	Timeout int  `yaml:"timeout"`
	Enabled bool `yaml:"enabled"`
}

func TestDecoderSuccess(t *testing.T) {
	t.Parallel()

	input := "name: test\nsettings:\n  timeout: 30\n  enabled: true\n"
	dec := strictyaml.New(
		strictyaml.WithSchema[sampleRoot]("root field"),
	)

	var target sampleRoot
	if err := dec.Decode([]byte(input), &target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Name != "test" || target.Settings.Timeout != 30 {
		t.Errorf("decoded struct = %+v", target)
	}
}

func TestDecoderUnknownFieldWithSuggestion(t *testing.T) {
	t.Parallel()

	input := "name: test\nsettings:\n  time-out: 30\n"
	dec := strictyaml.New(
		strictyaml.WithSchema[sampleSettings]("setting"),
	)

	var target sampleRoot
	err := dec.Decode([]byte(input), &target)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}

	var decErr *strictyaml.DecodeError
	if !errors.As(err, &decErr) {
		t.Fatalf("expected *strictyaml.DecodeError, got %T: %v", err, err)
	}

	if !decErr.HasUnknownFields() {
		t.Fatal("expected HasUnknownFields() to be true")
	}
	if len(decErr.UnknownFields) != 1 {
		t.Fatalf("expected 1 unknown field, got %d", len(decErr.UnknownFields))
	}

	uf := decErr.UnknownFields[0]
	if uf.Field != "time-out" {
		t.Errorf("Field = %q, want 'time-out'", uf.Field)
	}
	if uf.Line != 3 {
		t.Errorf("Line = %d, want 3", uf.Line)
	}
	if uf.Label != "setting" {
		t.Errorf("Label = %q, want 'setting'", uf.Label)
	}
	if uf.Suggestion != "timeout" {
		t.Errorf("Suggestion = %q, want 'timeout'", uf.Suggestion)
	}

	defaultStr := decErr.Error()
	if !strings.Contains(defaultStr, `unknown setting "time-out" (line 3)`) {
		t.Errorf("default Error() string should describe the field, got: %q", defaultStr)
	}
	if !strings.Contains(defaultStr, `did you mean "timeout"?`) {
		t.Errorf("default Error() string should include suggestion, got: %q", defaultStr)
	}
}

func TestDecoderUnknownFieldNoSuggestion(t *testing.T) {
	t.Parallel()

	input := "name: test\nbogus_key: 123\n"
	dec := strictyaml.New(
		strictyaml.WithSchema[sampleRoot]("root field"),
	)

	var target sampleRoot
	err := dec.Decode([]byte(input), &target)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}

	var decErr *strictyaml.DecodeError
	if !errors.As(err, &decErr) {
		t.Fatalf("expected *strictyaml.DecodeError, got %T: %v", err, err)
	}

	if len(decErr.UnknownFields) != 1 {
		t.Fatalf("expected 1 unknown field, got %d", len(decErr.UnknownFields))
	}

	uf := decErr.UnknownFields[0]
	if uf.Field != "bogus_key" {
		t.Errorf("Field = %q, want 'bogus_key'", uf.Field)
	}
	if uf.Suggestion != "" {
		t.Errorf("expected no suggestion, got %q", uf.Suggestion)
	}
}

func TestDecoderUnknownContextFallback(t *testing.T) {
	t.Parallel()

	input := "unknown_top: true\n"
	dec := strictyaml.New()

	var target sampleRoot
	err := dec.Decode([]byte(input), &target)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}

	var decErr *strictyaml.DecodeError
	if !errors.As(err, &decErr) {
		t.Fatalf("expected *strictyaml.DecodeError, got %T: %v", err, err)
	}

	if len(decErr.UnknownFields) != 1 {
		t.Fatalf("expected 1 unknown field, got %d", len(decErr.UnknownFields))
	}

	uf := decErr.UnknownFields[0]
	if uf.Label != "key" {
		t.Errorf("expected fallback label 'key', got %q", uf.Label)
	}
}

func TestDecoderCustomThreshold(t *testing.T) {
	t.Parallel()

	input := "name: test\nsettings:\n  time-out: 30\n"
	// Edit distance between "time-out" and "timeout" is 1.
	dec := strictyaml.New(
		strictyaml.WithSchema[sampleSettings]("setting"),
		strictyaml.WithSuggestionThreshold(1),
	)

	var target sampleRoot
	err := dec.Decode([]byte(input), &target)
	if err == nil {
		t.Fatal("expected error")
	}

	var decErr *strictyaml.DecodeError
	if !errors.As(err, &decErr) || len(decErr.UnknownFields) != 1 {
		t.Fatalf("expected 1 unknown field error, got: %v", err)
	}
	if decErr.UnknownFields[0].Suggestion != "timeout" {
		t.Errorf("expected suggestion 'timeout', got %q", decErr.UnknownFields[0].Suggestion)
	}
}
