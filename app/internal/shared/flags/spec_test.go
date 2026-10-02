package flags_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/spf13/pflag"
)

func newFlags() *pflag.FlagSet {
	return pflag.NewFlagSet("test", pflag.ContinueOnError)
}

func TestSpecHelpText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec flags.Spec[string]
		want string
	}{
		{
			name: "with env var",
			spec: flags.Spec[string]{
				Name:  "env",
				Env:   "ENVX_ENV",
				Usage: "target environment",
			},
			want: "target environment (env: ENVX_ENV)",
		},
		{
			name: "without env var",
			spec: flags.Spec[string]{
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

func TestGet(t *testing.T) {
	t.Parallel()

	strSpec := flags.Spec[string]{Name: "env", Default: "default-env"}
	boolSpec := flags.Spec[bool]{Name: "overload", Default: false}
	sliceSpec := flags.Spec[[]string]{Name: "tags", Default: []string{"default-tag"}}

	t.Run("unregistered returns spec.Default", func(t *testing.T) {
		fs := newFlags()
		if got := strSpec.Get(fs); got != "default-env" {
			t.Errorf("Get() = %q, want default-env", got)
		}
		if got := boolSpec.Get(fs); got != false {
			t.Errorf("Get() = %v, want false", got)
		}
		if got := sliceSpec.Get(fs); len(got) != 1 || got[0] != "default-tag" {
			t.Errorf("Get() = %v, want [default-tag]", got)
		}
	})

	t.Run("unchanged returns default", func(t *testing.T) {
		fs := newFlags()
		flags.Bind(fs, &strSpec)
		flags.Bind(fs, &boolSpec)
		flags.Bind(fs, &sliceSpec)

		if got := strSpec.Get(fs); got != "default-env" {
			t.Errorf("Get() = %q, want default-env", got)
		}
		if got := boolSpec.Get(fs); got != false {
			t.Errorf("Get() = %v, want false", got)
		}
	})

	t.Run("changed returns parsed value", func(t *testing.T) {
		fs := newFlags()
		flags.Bind(fs, &strSpec)
		flags.Bind(fs, &boolSpec)
		flags.Bind(fs, &sliceSpec)

		args := []string{"--env", "prod", "--overload", "--tags", "t1,t2"}
		if err := fs.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}

		if got := strSpec.Get(fs); got != "prod" {
			t.Errorf("got = %v, want prod", got)
		}
		if got := boolSpec.Get(fs); !got {
			t.Errorf("got = %v, want true", got)
		}
		if got := sliceSpec.Get(fs); len(got) != 2 || got[0] != "t1" || got[1] != "t2" {
			t.Errorf("got = %v, want [t1 t2]", got)
		}
	})
}

func TestGetOpt(t *testing.T) {
	strSpec := flags.Spec[string]{Name: "env", Default: "default-env"}
	boolSpec := flags.Spec[bool]{Name: "overload", Default: false}
	sliceSpec := flags.Spec[[]string]{Name: "tags"}

	t.Run("unchanged returns nil", func(t *testing.T) {
		fs := newFlags()
		flags.Bind(fs, &strSpec)
		flags.Bind(fs, &boolSpec)
		flags.Bind(fs, &sliceSpec)

		if got := strSpec.GetOpt(fs); got != nil {
			t.Errorf("expected nil for unchanged string, got %v", *got)
		}
		if got := boolSpec.GetOpt(fs); got != nil {
			t.Errorf("expected nil for unchanged bool, got %v", *got)
		}
		if got := sliceSpec.GetOpt(fs); got != nil {
			t.Errorf("expected nil for unchanged slice, got %v", got)
		}
	})

	t.Run("unregistered returns nil", func(t *testing.T) {
		fs := newFlags()
		if got := strSpec.GetOpt(fs); got != nil {
			t.Errorf("expected nil for unregistered flag, got %v", *got)
		}
	})

	t.Run("changed returns pointer to value", func(t *testing.T) {
		fs := newFlags()
		flags.Bind(fs, &strSpec)
		flags.Bind(fs, &boolSpec)
		flags.Bind(fs, &sliceSpec)

		args := []string{"--env", "prod", "--overload", "--tags", "t1,t2"}
		if err := fs.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}

		gotStr := strSpec.GetOpt(fs)
		if gotStr == nil || *gotStr != "prod" {
			t.Errorf("gotStr = %v, want prod", gotStr)
		}

		gotBool := boolSpec.GetOpt(fs)
		if gotBool == nil || !*gotBool {
			t.Errorf("gotBool = %v, want true", gotBool)
		}

		gotSlice := sliceSpec.GetOpt(fs)
		if gotSlice == nil || len(*gotSlice) != 2 ||
			(*gotSlice)[0] != "t1" || (*gotSlice)[1] != "t2" {
			t.Errorf("gotSlice = %v, want [t1 t2]", gotSlice)
		}
	})

	t.Run("environment variable fallback when flag unchanged", func(t *testing.T) {
		t.Setenv("TEST_FLAG_ENV", "staging")
		t.Setenv("TEST_FLAG_OVERLOAD", "true")
		t.Setenv("TEST_FLAG_TAGS", "alpha,beta")

		envStrSpec := flags.Spec[string]{
			Name: "env", Env: "TEST_FLAG_ENV", Default: "dev",
		}
		envBoolSpec := flags.Spec[bool]{
			Name: "overload", Env: "TEST_FLAG_OVERLOAD", Default: false,
		}
		envSliceSpec := flags.Spec[[]string]{
			Name: "tags", Env: "TEST_FLAG_TAGS",
		}

		fs := newFlags()
		flags.Bind(fs, &envStrSpec)
		flags.Bind(fs, &envBoolSpec)
		flags.Bind(fs, &envSliceSpec)

		gotStr := envStrSpec.GetOpt(fs)
		if gotStr == nil || *gotStr != "staging" {
			t.Errorf("gotStr = %v, want staging", gotStr)
		}
		if got := envStrSpec.Get(fs); got != "staging" {
			t.Errorf("Get() = %q, want staging", got)
		}

		gotBool := envBoolSpec.GetOpt(fs)
		if gotBool == nil || !*gotBool {
			t.Errorf("gotBool = %v, want true", gotBool)
		}
		if got := envBoolSpec.Get(fs); !got {
			t.Errorf("Get() = %v, want true", got)
		}

		gotSlice := envSliceSpec.GetOpt(fs)
		if gotSlice == nil || len(*gotSlice) != 2 ||
			(*gotSlice)[0] != "alpha" || (*gotSlice)[1] != "beta" {
			t.Errorf("gotSlice = %v, want [alpha beta]", gotSlice)
		}
		if got := envSliceSpec.Get(fs); len(got) != 2 ||
			got[0] != "alpha" || got[1] != "beta" {
			t.Errorf("Get() = %v, want [alpha beta]", got)
		}
	})

	t.Run("flag overrides environment variable fallback", func(t *testing.T) {
		t.Setenv("TEST_FLAG_ENV", "env-value")
		t.Setenv("TEST_FLAG_OVERLOAD", "false")

		envStrSpec := flags.Spec[string]{
			Name: "env", Env: "TEST_FLAG_ENV", Default: "dev",
		}
		envBoolSpec := flags.Spec[bool]{
			Name: "overload", Env: "TEST_FLAG_OVERLOAD", Default: false,
		}

		fs := newFlags()
		flags.Bind(fs, &envStrSpec)
		flags.Bind(fs, &envBoolSpec)

		if err := fs.Parse([]string{"--env", "cli-value", "--overload=true"}); err != nil {
			t.Fatalf("parse: %v", err)
		}

		gotStr := envStrSpec.GetOpt(fs)
		if gotStr == nil || *gotStr != "cli-value" {
			t.Errorf("GetOpt() = %v, want cli-value", gotStr)
		}
		if got := envStrSpec.Get(fs); got != "cli-value" {
			t.Errorf("Get() = %q, want cli-value", got)
		}

		gotBool := envBoolSpec.GetOpt(fs)
		if gotBool == nil || !*gotBool {
			t.Errorf("GetOpt() = %v, want true", gotBool)
		}
		if got := envBoolSpec.Get(fs); !got {
			t.Errorf("Get() = %v, want true", got)
		}
	})

	t.Run("empty or invalid env var falls through", func(t *testing.T) {
		t.Setenv("TEST_FLAG_ENV", "")
		t.Setenv("TEST_FLAG_OVERLOAD", "not-a-bool")

		envStrSpec := flags.Spec[string]{
			Name: "env", Env: "TEST_FLAG_ENV", Default: "dev",
		}
		envBoolSpec := flags.Spec[bool]{
			Name: "overload", Env: "TEST_FLAG_OVERLOAD", Default: false,
		}

		fs := newFlags()
		flags.Bind(fs, &envStrSpec)
		flags.Bind(fs, &envBoolSpec)

		if got := envStrSpec.GetOpt(fs); got != nil {
			t.Errorf("GetOpt() = %v, want nil for empty env var", got)
		}
		if got := envStrSpec.Get(fs); got != "dev" {
			t.Errorf("Get() = %q, want default dev", got)
		}

		if got := envBoolSpec.GetOpt(fs); got != nil {
			t.Errorf("GetOpt() = %v, want nil for invalid bool env var", got)
		}
		if got := envBoolSpec.Get(fs); got != false {
			t.Errorf("Get() = %v, want default false", got)
		}
	})
}
