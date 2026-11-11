package external_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/external"
)

func TestInspectorCheckAvailable(t *testing.T) {
	t.Parallel()

	missing := errors.New("missing CLI")
	inspector := external.NewInspector([]external.Registration{
		{Provider: &fakeProvider{scheme: "op", availableErr: missing}},
		{Provider: &fakeProvider{scheme: "bws"}},
	})

	if err := inspector.CheckAvailable("op"); !errors.Is(err, missing) {
		t.Errorf("op: err = %v, want %v", err, missing)
	}
	if err := inspector.CheckAvailable("bws"); err != nil {
		t.Errorf("bws: err = %v, want nil", err)
	}
	if err := inspector.CheckAvailable("passwordstate"); err != nil {
		t.Errorf("undeclared: err = %v, want nil", err)
	}
}
