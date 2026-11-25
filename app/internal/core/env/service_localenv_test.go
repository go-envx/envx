package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-envx/envx/app/internal/core/workspace"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// fileHostOf returns a lookup over a local env file with content and the given
// process KEY=value pairs, plus the file path.
func fileHostOf(
	t *testing.T, content string, environ ...string,
) (lookup *hostenv.Lookup, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "envx.local.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return hostenv.NewLookup(hostenv.Options{
		File:    path,
		Environ: func() []string { return environ },
	}), path
}

// TestApplyHostLocalFile verifies a file value carries its path as provenance
// and union adds only process-only keys.
func TestApplyHostLocalFile(t *testing.T) {
	t.Parallel()

	const file = "/work/envx.local.yaml"
	baseSource := Source{Path: appInclude + ".yaml", Key: "host"}
	fileSource := Source{Path: file, Key: "HOST", Kind: SourceLocalEnv}
	host := map[string]hostenv.Var{
		"HOST":      {Value: "from-file", File: file},
		"FILE_ONLY": {Value: "f", File: file},
		"PROC_ONLY": {Value: "p"},
	}

	tests := []struct {
		name       string
		overload   bool
		union      bool
		wantOrigin Origin
	}{
		{
			name:       "file winner",
			wantOrigin: Origin{Winner: fileSource, Shadowed: []Source{baseSource}},
		},
		{
			name: "file shadowed under overload", overload: true,
			wantOrigin: Origin{Winner: baseSource, Shadowed: []Source{fileSource}},
		},
		{
			name: "union skips file-only key", union: true,
			wantOrigin: Origin{Winner: fileSource, Shadowed: []Source{baseSource}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			namespaces := newFakeNamespaces().base(t, appInclude, "host: from-ns")
			merged, err := mergeFixture(
				t, namespaces, "development", []string{appInclude}, resolvedSettings{},
			)
			if err != nil {
				t.Fatalf("merge: %v", err)
			}
			merged.applyHost(operation{
				host: host, settings: resolvedSettings{overload: tt.overload},
			}, tt.union)

			if got := merged.origins["HOST"]; !originEqual(got, tt.wantOrigin) {
				t.Errorf("HOST origin = %+v, want %+v", got, tt.wantOrigin)
			}
			if _, ok := merged.values["FILE_ONLY"]; ok {
				t.Error("FILE_ONLY present, want skipped")
			}
			if _, ok := merged.values["PROC_ONLY"]; ok != tt.union {
				t.Errorf("PROC_ONLY present = %v, want %v", ok, tt.union)
			}
		})
	}
}

// TestHostFallbackRecordsFile verifies the fallback winner names the file.
func TestHostFallbackRecordsFile(t *testing.T) {
	t.Parallel()

	const file = "/work/envx.local.yaml"
	prev := Origin{Winner: Source{Path: "ns.yaml", Key: "k"}}
	got := (&Service{}).hostFallback(prev, "K", file)
	want := Origin{
		Winner:   Source{Path: file, Key: "K", Kind: SourceLocalEnv},
		Shadowed: []Source{prev.Winner},
	}
	if !originEqual(got, want) {
		t.Errorf("origin = %+v, want %+v", got, want)
	}
}

// TestMaterializeSubstitutesFileValue verifies {{VAR}} resolves from the file
// and a failed key falls back to its file value with the file as source.
func TestMaterializeSubstitutesFileValue(t *testing.T) {
	t.Parallel()

	host, file := fileHostOf(t, "DANGLING=from-file\nREGION=eu\n")
	opener := &fakeOpener{}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces: newFakeNamespaces().base(t, appInclude, `
			url: "api.{{REGION}}"
			dangling: secret://db/missing
		`),
		HostEnvLookup:    host,
		NewValueResolver: opener.open,
	})
	result, err := s.Materialize(MaterializeParams{
		Options: workspace.Settings{Overload: ptr(true)}, IgnoreErrors: true,
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	all := result.Environment.All()
	if all["URL"] != "api.eu" || all["DANGLING"] != "from-file" {
		t.Errorf("environment = %v", all)
	}
	origin, ok := result.Environment.Origin("DANGLING")
	if !ok || origin.Winner.Path != file || origin.Winner.Kind != SourceLocalEnv {
		t.Errorf("DANGLING origin = %+v, want host winner from %s", origin, file)
	}
}

// TestRunLocalEnvFile verifies a file value overrides a declared key and a
// file-only key never reaches the child.
func TestRunLocalEnvFile(t *testing.T) {
	t.Parallel()

	host, _ := fileHostOf(t, "DECLARED=from-file\nFILE_ONLY=f\n", "PROC_ONLY=p")
	executor := &fakeExecutor{}
	opener := &fakeOpener{}
	s := newService(t, testWorkspace(), Dependencies{
		EnvNamespaces:    newFakeNamespaces().base(t, appInclude, "declared: ns"),
		HostEnvLookup:    host,
		NewValueResolver: opener.open,
		ProcessRunner:    executor.run,
	})
	if err := s.Run(RunParams{Args: []string{"x"}}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	env := executor.command.Env
	if env["DECLARED"] != "from-file" || env["PROC_ONLY"] != "p" {
		t.Errorf("env = %v", env)
	}
	if _, ok := env["FILE_ONLY"]; ok {
		t.Error("FILE_ONLY reached the child")
	}
}
