package flags_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/shared/flags"
)

func TestBind(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		spec := flags.Spec[string]{
			Name:  "output",
			Short: "o",
			Usage: "output format",
		}
		fs := newFlags()
		flags.Bind(fs, &spec)
		if err := fs.Parse([]string{"--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := spec.Get(fs); got != "json" {
			t.Errorf("output = %q, want json", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		spec := flags.Spec[bool]{
			Name:  "verbose",
			Short: "v",
			Usage: "verbose output",
		}
		fs := newFlags()
		flags.Bind(fs, &spec)
		if err := fs.Parse([]string{"--verbose"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := spec.Get(fs); !got {
			t.Error("verbose = false, want true")
		}
	})

	t.Run("string slice", func(t *testing.T) {
		spec := flags.Spec[[]string]{
			Name:  "item",
			Usage: "items list",
		}
		fs := newFlags()
		flags.Bind(fs, &spec)
		if err := fs.Parse([]string{"--item", "a", "--item", "b"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		items := spec.Get(fs)
		if len(items) != 2 || items[0] != "a" || items[1] != "b" {
			t.Errorf("items = %v, want [a b]", items)
		}
	})
}
