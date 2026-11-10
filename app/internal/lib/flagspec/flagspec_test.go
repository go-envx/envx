package flagspec_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/flagspec"
	"github.com/spf13/cobra"
)

// parseCommand returns a command whose flags are registered by bind and parsed
// from args, with its error stream captured.
func parseCommand(
	t *testing.T, bind func(*cobra.Command), args ...string,
) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)
	bind(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return cmd, stderr
}

func TestFlagSpecValue(t *testing.T) {
	t.Parallel()

	outputSpec := flagspec.Spec[string]{
		Name:    "output",
		Short:   "o",
		Usage:   "output format",
		Default: "text",
	}
	verboseSpec := flagspec.Spec[bool]{
		Name:    "verbose",
		Short:   "v",
		Usage:   "print verbose output",
		Default: false,
	}
	tagsSpec := flagspec.Spec[[]string]{
		Name:    "tags",
		Usage:   "filter tags",
		Default: []string{"default"},
	}

	bind := func(cmd *cobra.Command) {
		outputSpec.Bind(cmd)
		verboseSpec.Bind(cmd)
		tagsSpec.Bind(cmd)
	}

	t.Run("unchanged returns default", func(t *testing.T) {
		t.Parallel()
		cmd, _ := parseCommand(t, bind)
		if got := outputSpec.Value(cmd); got != "text" {
			t.Errorf("output = %q, want text", got)
		}
		if got := verboseSpec.Value(cmd); got != false {
			t.Errorf("verbose = %v, want false", got)
		}
		if got := tagsSpec.Value(cmd); !slices.Equal(got, []string{"default"}) {
			t.Errorf("tags = %v, want [default]", got)
		}
	})

	t.Run("changed returns parsed value", func(t *testing.T) {
		t.Parallel()
		cmd, _ := parseCommand(t, bind, "-o", "json", "-v", "--tags", "a,b")
		if got := outputSpec.Value(cmd); got != "json" {
			t.Errorf("output = %q, want json", got)
		}
		if got := verboseSpec.Value(cmd); !got {
			t.Errorf("verbose = %v, want true", got)
		}
		if got := tagsSpec.Value(cmd); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("tags = %v, want [a b]", got)
		}
	})

	t.Run("unregistered returns default", func(t *testing.T) {
		t.Parallel()
		spec := flagspec.Spec[string]{Name: "missing", Default: "dflt"}
		cmd, _ := parseCommand(t, bind)
		if got := spec.Value(cmd); got != "dflt" {
			t.Errorf("value = %q, want dflt", got)
		}
	})
}

func TestFlagSpecBindPersistent(t *testing.T) {
	t.Parallel()

	spec := flagspec.Spec[string]{Name: "file", Default: "dflt"}
	root := &cobra.Command{Use: "root"}
	spec.BindPersistent(root)

	var got string
	child := &cobra.Command{
		Use: "child",
		Run: func(cmd *cobra.Command, _ []string) {
			got = spec.Value(cmd)
		},
	}
	root.AddCommand(child)
	root.SetArgs([]string{"child", "--file", "inherited.yaml"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got != "inherited.yaml" {
		t.Errorf("value = %q, want inherited.yaml", got)
	}
}

func TestFlagSpecLookupEnvironment(t *testing.T) {
	fileSpec := flagspec.Spec[string]{
		Name:  "file",
		Env:   "TEST_CONFIG_FILE",
		Usage: "path to configuration file",
	}

	t.Run("help names the variable", func(t *testing.T) {
		cmd, _ := parseCommand(t, fileSpec.Bind)
		usage := cmd.Flags().Lookup(fileSpec.Name).Usage
		if !strings.HasSuffix(usage, "(env: TEST_CONFIG_FILE)") {
			t.Errorf("usage = %q, want env hint", usage)
		}
	})

	t.Run("help without env has no suffix", func(t *testing.T) {
		plainSpec := flagspec.Spec[string]{
			Name:  "plain",
			Usage: "plain description",
		}
		cmd, _ := parseCommand(t, plainSpec.Bind)
		usage := cmd.Flags().Lookup(plainSpec.Name).Usage
		if usage != "plain description" {
			t.Errorf("usage = %q, want plain description", usage)
		}
	})

	t.Run("variable applies when flag unchanged", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_FILE", "from-env.yaml")
		cmd, _ := parseCommand(t, fileSpec.Bind)
		if got := fileSpec.Lookup(cmd); got == nil || *got != "from-env.yaml" {
			t.Errorf("lookup = %v, want from-env.yaml", got)
		}
		if got := fileSpec.Value(cmd); got != "from-env.yaml" {
			t.Errorf("value = %q, want from-env.yaml", got)
		}
	})

	t.Run("flag wins over variable", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_FILE", "from-env.yaml")
		cmd, _ := parseCommand(t, fileSpec.Bind, "--file", "from-flag.yaml")
		if got := fileSpec.Value(cmd); got != "from-flag.yaml" {
			t.Errorf("value = %q, want from-flag.yaml", got)
		}
	})

	t.Run("empty variable is unset", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_FILE", "")
		cmd, _ := parseCommand(t, fileSpec.Bind)
		if got := fileSpec.Lookup(cmd); got != nil {
			t.Errorf("lookup = %q, want nil", *got)
		}
	})

	t.Run("unregistered flag ignores variable", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_FILE", "from-env.yaml")
		cmd, _ := parseCommand(t, func(*cobra.Command) {})
		if got := fileSpec.Lookup(cmd); got != nil {
			t.Errorf("lookup = %q, want nil", *got)
		}
	})

	t.Run("invalid variable warns on command stream", func(t *testing.T) {
		t.Setenv("TEST_FLAG_STRICT", "not-a-bool")
		spec := flagspec.Spec[bool]{Name: "strict", Env: "TEST_FLAG_STRICT"}
		cmd, stderr := parseCommand(t, spec.Bind)
		if got := spec.Lookup(cmd); got != nil {
			t.Errorf("lookup = %v, want nil", *got)
		}
		if !strings.Contains(stderr.String(), "TEST_FLAG_STRICT") {
			t.Errorf("stderr = %q, want warning naming the variable", stderr)
		}
	})

	t.Run("list variable splits on commas", func(t *testing.T) {
		t.Setenv("TEST_FLAG_TAGS", "a,b,c")
		spec := flagspec.Spec[[]string]{Name: "tags", Env: "TEST_FLAG_TAGS"}
		cmd, _ := parseCommand(t, spec.Bind)
		if got := spec.Value(cmd); !slices.Equal(got, []string{"a", "b", "c"}) {
			t.Errorf("value = %v, want [a b c]", got)
		}
	})

	t.Run("unset variable returns nil", func(t *testing.T) {
		spec := flagspec.Spec[string]{Name: "file", Env: "TEST_NONEXISTENT_VAR"}
		cmd, _ := parseCommand(t, spec.Bind)
		if got := spec.Lookup(cmd); got != nil {
			t.Errorf("lookup = %v, want nil", *got)
		}
	})

	t.Run("bool variable applies when valid", func(t *testing.T) {
		t.Setenv("TEST_FLAG_BOOL", "true")
		spec := flagspec.Spec[bool]{Name: "verbose", Env: "TEST_FLAG_BOOL"}
		cmd, _ := parseCommand(t, spec.Bind)
		if got := spec.Lookup(cmd); got == nil || !*got {
			t.Errorf("lookup = %v, want true", got)
		}
	})

	t.Run("type mismatch on changed flag warns on error stream", func(t *testing.T) {
		spec := flagspec.Spec[string]{Name: "count"}
		cmd := &cobra.Command{Use: "test"}
		stderr := new(bytes.Buffer)
		cmd.SetErr(stderr)
		cmd.Flags().Int("count", 0, "count flag")
		if err := cmd.ParseFlags([]string{"--count=5"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := spec.Lookup(cmd); got != nil {
			t.Errorf("lookup = %v, want nil", *got)
		}
		if !strings.Contains(stderr.String(), "warning: invalid flag") {
			t.Errorf("stderr = %q, want invalid flag warning", stderr)
		}
	})
}
