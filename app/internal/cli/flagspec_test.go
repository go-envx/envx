package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// parseCommand returns a command whose flags are registered by bind and parsed
// from args, with its error stream captured.
func parseCommand(
	t *testing.T, bind func(*pflag.FlagSet), args ...string,
) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)
	bind(cmd.Flags())
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return cmd, stderr
}

func TestFlagSpecValue(t *testing.T) {
	t.Parallel()

	bind := func(fs *pflag.FlagSet) {
		outputFlag.bind(fs)
		verboseFlag.bind(fs)
		revealFlag.bind(fs)
	}

	t.Run("unchanged returns fallback", func(t *testing.T) {
		t.Parallel()
		cmd, _ := parseCommand(t, bind)
		if got := outputFlag.value(cmd); got != "" {
			t.Errorf("output = %q, want empty", got)
		}
		if verboseFlag.value(cmd) || revealFlag.value(cmd) {
			t.Error("bool flags = true, want false")
		}
	})

	t.Run("changed returns parsed value", func(t *testing.T) {
		t.Parallel()
		cmd, _ := parseCommand(t, bind, "-o", "json", "-v", "--reveal")
		if got := outputFlag.value(cmd); got != "json" {
			t.Errorf("output = %q, want json", got)
		}
		if !verboseFlag.value(cmd) || !revealFlag.value(cmd) {
			t.Error("bool flags = false, want true")
		}
	})

	t.Run("unregistered returns fallback", func(t *testing.T) {
		t.Parallel()
		spec := flagSpec[string]{name: "missing", fallback: "dflt"}
		cmd, _ := parseCommand(t, bind)
		if got := spec.value(cmd); got != "dflt" {
			t.Errorf("value = %q, want dflt", got)
		}
	})
}

func TestFlagSpecLookupEnvironment(t *testing.T) {
	t.Run("help names the variable", func(t *testing.T) {
		cmd, _ := parseCommand(t, configFlag.bind)
		usage := cmd.Flags().Lookup(configFlag.name).Usage
		if !strings.HasSuffix(usage, "(env: ENVX_CONFIG)") {
			t.Errorf("usage = %q, want env hint", usage)
		}
	})

	t.Run("variable applies when flag unchanged", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from-env.yaml")
		cmd, _ := parseCommand(t, configFlag.bind)
		if got := configFlag.lookup(cmd); got == nil || *got != "from-env.yaml" {
			t.Errorf("lookup = %v, want from-env.yaml", got)
		}
	})

	t.Run("flag wins over variable", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from-env.yaml")
		cmd, _ := parseCommand(t, configFlag.bind, "--config", "from-flag.yaml")
		if got := configFlag.value(cmd); got != "from-flag.yaml" {
			t.Errorf("value = %q, want from-flag.yaml", got)
		}
	})

	t.Run("empty variable is unset", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "")
		cmd, _ := parseCommand(t, configFlag.bind)
		if got := configFlag.lookup(cmd); got != nil {
			t.Errorf("lookup = %q, want nil", *got)
		}
	})

	t.Run("unregistered flag ignores variable", func(t *testing.T) {
		t.Setenv("ENVX_CONFIG", "from-env.yaml")
		cmd, _ := parseCommand(t, func(*pflag.FlagSet) {})
		if got := configFlag.lookup(cmd); got != nil {
			t.Errorf("lookup = %q, want nil", *got)
		}
	})

	t.Run("invalid variable warns on command stream", func(t *testing.T) {
		t.Setenv("ENVX_TEST_STRICT", "not-a-bool")
		spec := flagSpec[bool]{name: "strict", envVar: "ENVX_TEST_STRICT"}
		cmd, stderr := parseCommand(t, spec.bind)
		if got := spec.lookup(cmd); got != nil {
			t.Errorf("lookup = %v, want nil", *got)
		}
		if !strings.Contains(stderr.String(), "ENVX_TEST_STRICT") {
			t.Errorf("stderr = %q, want warning naming the variable", stderr)
		}
	})

	t.Run("list variable splits on commas", func(t *testing.T) {
		t.Setenv("ENVX_TEST_TAGS", "a,b")
		spec := flagSpec[[]string]{name: "tags", envVar: "ENVX_TEST_TAGS"}
		cmd, _ := parseCommand(t, spec.bind)
		if got := spec.value(cmd); !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("value = %v, want [a b]", got)
		}
	})
}
