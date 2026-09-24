package cliflags_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/utils/cliflags"
	"github.com/spf13/pflag"
)

func newFlags() *pflag.FlagSet {
	return pflag.NewFlagSet("test", pflag.ContinueOnError)
}

func TestSpecHelpText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec cliflags.FlagSpec
		want string
	}{
		{
			name: "with env var",
			spec: cliflags.FlagSpec{
				Name:  "env",
				Env:   "ENVX_ENV",
				Usage: "target environment",
			},
			want: "target environment (env: ENVX_ENV)",
		},
		{
			name: "without env var",
			spec: cliflags.FlagSpec{
				Name:  "output",
				Usage: "output format: table|json",
			},
			want: "output format: table|json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.spec.HelpText(); got != tt.want {
				t.Errorf("HelpText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBindString(t *testing.T) {
	t.Parallel()

	var output string
	spec := cliflags.FlagSpec{
		Name:  "output",
		Short: "o",
		Usage: "output format",
	}
	fs := newFlags()
	cliflags.BindString(fs, &output, &spec)
	if err := fs.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if output != "json" {
		t.Errorf("output = %q, want json", output)
	}
}

func TestBindBool(t *testing.T) {
	t.Parallel()

	var verbose bool
	spec := cliflags.FlagSpec{
		Name:  "verbose",
		Short: "v",
		Usage: "verbose output",
	}
	fs := newFlags()
	cliflags.BindBool(fs, &verbose, &spec)
	if err := fs.Parse([]string{"--verbose"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !verbose {
		t.Error("verbose = false, want true")
	}
}

func TestBindStringSlice(t *testing.T) {
	t.Parallel()

	var items []string
	spec := cliflags.FlagSpec{
		Name:  "item",
		Usage: "items list",
	}
	fs := newFlags()
	cliflags.BindStringSlice(fs, &items, &spec)
	if err := fs.Parse([]string{"--item", "a", "--item", "b"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Errorf("items = %v, want [a b]", items)
	}
}
