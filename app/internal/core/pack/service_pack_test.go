package pack

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// TestPackWritesPerProjectDirectories verifies each project's selected files
// land in its own directory beside a root manifest and store, and a namespace
// both projects include is copied into each.
func TestPackWritesPerProjectDirectories(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	f.write(t, "envx.keys", "shared: PRIVATE\n")
	out := filepath.Join(t.TempDir(), "dist")

	result := f.pack(t, PackParams{Environments: []string{"production"}, OutDir: out})

	want := []string{
		"api/api.yaml",
		"api/app.production.yaml",
		"api/app.yaml",
		"envx.yaml",
		"secrets.yaml",
		"web/app.production.yaml",
		"web/app.yaml",
	}
	if got := listFiles(t, out); !slices.Equal(got, want) {
		t.Errorf("bundle files = %v, want %v", got, want)
	}
	if !slices.Equal(result.Files, want) {
		t.Errorf("result files = %v, want %v", result.Files, want)
	}
	if result.OutDir != out || result.ManifestFile != "envx.yaml" {
		t.Errorf("result = %+v, want OutDir %s and envx.yaml", result, out)
	}
	if !slices.Equal(result.Projects, []string{"api", "web"}) {
		t.Errorf("projects = %v, want [api web]", result.Projects)
	}
	if got := readFile(t, out, "api/app.production.yaml"); got != "A: prod\n" {
		t.Errorf("overlay content = %q, want copied verbatim", got)
	}
}

// TestPackRewritesManifestIncludes verifies includes point at per-project paths.
func TestPackRewritesManifestIncludes(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	out := filepath.Join(t.TempDir(), "dist")

	f.pack(t, PackParams{OutDir: out})

	projects := manifestProjects(t, out)
	if got := projects["api"]; !slices.Equal(got, []string{"api/app", "api/api"}) {
		t.Errorf("api includes = %v, want [api/app api/api]", got)
	}
	if got := projects["web"]; !slices.Equal(got, []string{"web/app"}) {
		t.Errorf("web includes = %v, want [web/app]", got)
	}
}

// TestPackNamesFiles verifies basename collisions across and within projects,
// base/overlay filename collisions, unsafe project names, and includes outside
// the workspace each produce distinct, safe bundle paths.
func TestPackNamesFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		projects map[string][]string
		files    map[string]string
		want     map[string][]string
		contents map[string]string
	}{
		{
			name:     "same basename in two projects",
			projects: map[string][]string{"one": {"env/app"}, "two": {"svc/app"}},
			files:    map[string]string{"env/app.yaml": "A: 1\n", "svc/app.yaml": "A: 2\n"},
			want:     map[string][]string{"one": {"one/app"}, "two": {"two/app"}},
			contents: map[string]string{"one/app.yaml": "A: 1\n", "two/app.yaml": "A: 2\n"},
		},
		{
			name:     "same basename in one project",
			projects: map[string][]string{"one": {"env/app", "svc/app"}},
			files:    map[string]string{"env/app.yaml": "A: 1\n", "svc/app.yaml": "A: 2\n"},
			want:     map[string][]string{"one": {"one/app", "one/app-2"}},
			contents: map[string]string{"one/app.yaml": "A: 1\n", "one/app-2.yaml": "A: 2\n"},
		},
		{
			name:     "overlay and base filenames collide",
			projects: map[string][]string{"one": {"env/app", "svc/app.production"}},
			files: map[string]string{
				"env/app.yaml":            "A: base\n",
				"env/app.production.yaml": "A: overlay\n",
				"svc/app.production.yaml": "B: other\n",
			},
			want: map[string][]string{"one": {"one/app", "one/app.production-2"}},
			contents: map[string]string{
				"one/app.production.yaml":   "A: overlay\n",
				"one/app.production-2.yaml": "B: other\n",
			},
		},
		{
			name:     "unsafe project name",
			projects: map[string][]string{"team/api": {"env/app"}},
			files:    map[string]string{"env/app.yaml": "A: 1\n"},
			want:     map[string][]string{"team/api": {"team_api/app"}},
			contents: map[string]string{"team_api/app.yaml": "A: 1\n"},
		},
		{
			name:     "include outside the workspace",
			projects: map[string][]string{"escape": {"../outside"}},
			files:    map[string]string{"../outside.yaml": "X: 1\n"},
			want:     map[string][]string{"escape": {"escape/outside"}},
			contents: map[string]string{"escape/outside.yaml": "X: 1\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, "production")
			f.root = filepath.Join(f.root, "workspace")
			f.ws.Root = f.root
			f.ws.Path = f.write(t, "envx.yaml", manifestFor(tt.projects))
			for name, includes := range tt.projects {
				f.project(name, includes...)
			}
			for rel, body := range tt.files {
				f.write(t, rel, body)
			}
			out := filepath.Join(t.TempDir(), "dist")

			f.pack(t, PackParams{OutDir: out})

			got := manifestProjects(t, out)
			if !maps.EqualFunc(got, tt.want, slices.Equal[[]string]) {
				t.Errorf("manifest projects = %v, want %v", got, tt.want)
			}
			for rel, body := range tt.contents {
				if got := readFile(t, out, rel); got != body {
					t.Errorf("%s = %q, want %q", rel, got, body)
				}
			}
		})
	}
}

// TestPackSelectsEnvironmentsAndProjects verifies selections narrow overlays
// and projects, default to everything, and prune the manifest.
func TestPackSelectsEnvironmentsAndProjects(t *testing.T) {
	t.Parallel()
	t.Run("all by default", func(t *testing.T) {
		t.Parallel()
		f := newSampleFixture(t)
		out := filepath.Join(t.TempDir(), "dist")
		result := f.pack(t, PackParams{OutDir: out})
		if !slices.Equal(result.Environments, []string{"development", "production"}) {
			t.Errorf("environments = %v, want all declared", result.Environments)
		}
		present(t, out, "api/app.development.yaml")
		present(t, out, "api/app.production.yaml")
		present(t, out, "web/app.development.yaml")
	})
	t.Run("declared order", func(t *testing.T) {
		t.Parallel()
		f := newSampleFixture(t)
		out := filepath.Join(t.TempDir(), "dist")
		result := f.pack(t, PackParams{
			Environments: []string{"production", "development"},
			OutDir:       out,
		})
		if !slices.Equal(result.Environments, []string{"development", "production"}) {
			t.Errorf("environments = %v, want declared order", result.Environments)
		}
	})
	t.Run("one project", func(t *testing.T) {
		t.Parallel()
		f := newSampleFixture(t)
		out := filepath.Join(t.TempDir(), "dist")
		f.pack(t, PackParams{Projects: []string{"web"}, OutDir: out})
		present(t, out, "web/app.yaml")
		absent(t, out, "api")
		got := manifestProjects(t, out)
		if _, ok := got["api"]; ok || len(got) != 1 {
			t.Errorf("manifest projects = %v, want only web", got)
		}
	})
}

// TestPackRejectsInvalidRequests verifies selection and destination errors
// fail before anything is written.
func TestPackRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		params PackParams
		want   error
	}{
		{"no output", PackParams{}, ErrOutputRequired},
		{
			"unknown environment",
			PackParams{Environments: []string{"ghost"}},
			ErrEnvironmentNotDeclared,
		},
		{"unknown project", PackParams{Projects: []string{"nope"}}, ErrProjectNotDeclared},
		{
			"unknown environment with force",
			PackParams{Environments: []string{"ghost"}, Force: true},
			ErrEnvironmentNotDeclared,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newSampleFixture(t)
			parent := t.TempDir()
			if !errors.Is(tt.want, ErrOutputRequired) {
				tt.params.OutDir = filepath.Join(parent, "dist")
			}
			_, err := f.service(t).Pack(tt.params)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if entries, _ := os.ReadDir(parent); len(entries) != 0 {
				t.Errorf("output parent holds %d entries after a failed pack", len(entries))
			}
		})
	}
}

// TestPackFailsOnEmptyInclude verifies an include matching no file fails.
func TestPackFailsOnEmptyInclude(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	f.project("typo", "env/missing")

	_, err := f.service(t).Pack(PackParams{OutDir: filepath.Join(t.TempDir(), "dist")})
	if err == nil || !strings.Contains(err.Error(), "env/missing") {
		t.Fatalf("err = %v, want an empty-include error", err)
	}
}

// TestPackFiltersSecrets verifies the store keeps only referenced values from
// the union of selected projects, verbatim and without public keys.
func TestPackFiltersSecrets(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "production")
	f.project("one", "env/one")
	f.project("two", "env/two")
	f.write(t, "env/one.yaml", "A: secret://shared/alpha\nL: [secret://Shared/beta]\n")
	f.write(t, "env/two.yaml", "B:\n  C: secret://shared/beta\nD: secret://shared/ghost\n")
	f.useStore(t, "secrets.yaml", secrets.Group{
		Name:      "Shared",
		PublicKey: "PUBKEY",
		Values:    map[string]string{"alpha": "a0", "beta": "b0", "gamma": "g0"},
	})
	f.write(t, "envx.yaml", manifestFor(map[string][]string{
		"one": {"env/one"}, "two": {"env/two"},
	}))
	out := filepath.Join(t.TempDir(), "dist")

	f.pack(t, PackParams{OutDir: out})

	want := "Shared/alpha=a0\nShared/beta=b0\n"
	if got := readFile(t, out, "secrets.yaml"); got != want {
		t.Errorf("bundled store = %q, want %q", got, want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(out, "secrets.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("store mode = %v, want 0600", perm)
		}
	}
}

// TestPackOmitsStore verifies no store is written without a source store or
// without a stored referenced value.
func TestPackOmitsStore(t *testing.T) {
	t.Parallel()
	t.Run("no source store", func(t *testing.T) {
		t.Parallel()
		f := newSampleFixture(t)
		if err := os.Remove(f.ws.Secrets.SecretsPath); err != nil {
			t.Fatal(err)
		}
		f.store.readErr = errors.New("store must not be read")
		out := filepath.Join(t.TempDir(), "dist")
		f.pack(t, PackParams{OutDir: out})
		absent(t, out, "secrets.yaml")
	})
	t.Run("nothing referenced", func(t *testing.T) {
		t.Parallel()
		f := newSampleFixture(t)
		f.write(t, "env/app.yaml", "A: base\n")
		out := filepath.Join(t.TempDir(), "dist")
		f.pack(t, PackParams{OutDir: out})
		absent(t, out, "secrets.yaml")
		if f.store.exported != nil {
			t.Errorf("exported %v, want no export", f.store.exported)
		}
	})
}

// TestPackStandardizesManifest verifies the bundle uses default filenames and
// drops an explicit secrets path, removing a secrets block left empty.
func TestPackStandardizesManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		block    string
		keepKeys bool
	}{
		{
			"keeps other settings",
			"secrets:\n  path: private/vault.yaml\n  keys-path: k\n",
			true,
		},
		{"drops empty block", "secrets:\n  path: private/vault.yaml\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, "production")
			f.ws.Path = f.write(t, "config.yaml", "environments: [production]\n"+
				tt.block+"projects:\n  app:\n    includes: [env/app]\n")
			f.project("app", "env/app")
			f.write(t, "env/app.yaml", "TOKEN: secret://shared/token\n")
			f.useStore(t, "private/vault.yaml", secrets.Group{
				Name: "shared", Values: map[string]string{"token": "t0k"},
			})
			out := filepath.Join(t.TempDir(), "dist")

			f.pack(t, PackParams{OutDir: out})

			if got := listFiles(t, out); !slices.Equal(
				got, []string{"app/app.yaml", "envx.yaml", "secrets.yaml"},
			) {
				t.Errorf("bundle files = %v", got)
			}
			manifest := readFile(t, out, "envx.yaml")
			if strings.Contains(manifest, "vault.yaml") {
				t.Errorf("manifest kept the source secrets path:\n%s", manifest)
			}
			if got := strings.Contains(manifest, "keys-path:"); got != tt.keepKeys {
				t.Errorf("keys-path kept = %t, want %t:\n%s", got, tt.keepKeys, manifest)
			}
			if got := strings.Contains(manifest, "secrets:"); got != tt.keepKeys {
				t.Errorf("secrets block kept = %t, want %t:\n%s", got, tt.keepKeys, manifest)
			}
		})
	}
}

// TestPackOutputDirectoryPolicy verifies non-empty and non-directory
// destinations are refused without force and replaced with it.
func TestPackOutputDirectoryPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(t *testing.T, out string)
		force bool
		want  error
	}{
		{"missing", func(*testing.T, string) {}, false, nil},
		{"empty", func(t *testing.T, out string) {
			if err := os.MkdirAll(out, 0o750); err != nil {
				t.Fatal(err)
			}
		}, false, nil},
		{"non-empty", func(t *testing.T, out string) {
			writeFile(t, out, "leftover.txt", "keep\n")
		}, false, ErrOutputNotEmpty},
		{"file", func(t *testing.T, out string) {
			writeFile(t, filepath.Dir(out), filepath.Base(out), "file\n")
		}, false, ErrOutputNotDirectory},
		{"forced non-empty", func(t *testing.T, out string) {
			writeFile(t, out, "leftover.txt", "delete\n")
		}, true, nil},
		{"forced file", func(t *testing.T, out string) {
			writeFile(t, filepath.Dir(out), filepath.Base(out), "file\n")
		}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newSampleFixture(t)
			parent := t.TempDir()
			out := filepath.Join(parent, "dist")
			tt.setup(t, out)

			_, err := f.service(t).Pack(PackParams{OutDir: out, Force: tt.force})
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if entries, _ := os.ReadDir(parent); len(entries) != 1 {
				t.Errorf("output parent holds %d entries, want only dist", len(entries))
			}
			if tt.want != nil {
				return
			}
			present(t, out, "envx.yaml")
			absent(t, out, "leftover.txt")
		})
	}
}

// TestPackPreservesDestinationOnFailure verifies a forced pack that fails
// while staging leaves the previous bundle intact and no staging behind.
func TestPackPreservesDestinationOnFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fail func(f *fixture)
	}{
		{"read failure", func(f *fixture) { f.store.readErr = errors.New("boom") }},
		{"export failure", func(f *fixture) { f.store.exportErr = errors.New("boom") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newSampleFixture(t)
			tt.fail(f)
			parent := t.TempDir()
			out := filepath.Join(parent, "dist")
			writeFile(t, out, "envx.yaml", "previous\n")

			if _, err := f.service(t).Pack(PackParams{OutDir: out, Force: true}); err == nil {
				t.Fatal("Pack succeeded, want an error")
			}
			if got := readFile(t, out, "envx.yaml"); got != "previous\n" {
				t.Errorf("previous bundle changed to %q", got)
			}
			if entries, _ := os.ReadDir(parent); len(entries) != 1 {
				t.Errorf("output parent holds %d entries, want only dist", len(entries))
			}
		})
	}
}

// TestPackRejectsOutputContainingSources verifies a forced destination that is
// or contains a source, directly or through a symlink, is refused untouched.
func TestPackRejectsOutputContainingSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		out  func(t *testing.T, f *fixture) string
	}{
		{"workspace root", func(_ *testing.T, f *fixture) string { return f.root }},
		{"workspace parent", func(_ *testing.T, f *fixture) string {
			return filepath.Dir(f.root)
		}},
		{"namespace directory", func(_ *testing.T, f *fixture) string {
			return filepath.Join(f.root, "env")
		}},
		{"symlink to root", func(t *testing.T, f *fixture) string {
			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(f.root, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return link
		}},
		{"store directory", func(t *testing.T, f *fixture) string {
			f.useStore(t, "vault/secrets.yaml")
			return filepath.Join(f.root, "vault")
		}},
		{"key directory", func(t *testing.T, f *fixture) string {
			f.ws.Secrets.KeysPath = f.write(t, "keys/envx.keys", "shared: PRIVATE\n")
			return filepath.Join(f.root, "keys")
		}},
		{"unselected project directory", func(t *testing.T, f *fixture) string {
			f.project("other", "other/x")
			f.write(t, "other/x.yaml", "X: 1\n")
			return filepath.Join(f.root, "other")
		}},
		{"directory of a symlinked source", func(t *testing.T, f *fixture) string {
			shared := writeFile(t, t.TempDir(), "app.yaml", "A: shared\n")
			link := filepath.Join(f.root, "env", "app.yaml")
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(shared, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return filepath.Join(f.root, "env")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newSampleFixture(t)
			out := tt.out(t, f)

			_, err := f.service(t).Pack(PackParams{
				Projects: []string{"api"},
				OutDir:   out,
				Force:    true,
			})
			if !errors.Is(err, ErrOutputContainsSource) {
				t.Fatalf("err = %v, want ErrOutputContainsSource", err)
			}
			present(t, f.root, "envx.yaml")
			present(t, f.root, "env/app.yaml")
		})
	}
}

// TestPackForceKeepsDirectoryIdentity verifies a forced pack into an existing
// directory swaps its entries, keeping the directory and its mode.
func TestPackForceKeepsDirectoryIdentity(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	out := filepath.Join(t.TempDir(), "dist")
	// A mode other than the staged 0750 reveals a replaced directory.
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, out, "nested/leftover.txt", "delete\n")
	before, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}

	f.pack(t, PackParams{OutDir: out, Force: true})

	after, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || after.Mode() != before.Mode() {
		t.Errorf("destination replaced: mode %v -> %v", before.Mode(), after.Mode())
	}
	if got := listFiles(t, out); slices.Contains(got, "nested/leftover.txt") ||
		!slices.Contains(got, "envx.yaml") {
		t.Errorf("bundle files = %v, want fresh bundle only", got)
	}
	absent(t, out, scratchName)
}

// TestPackForceReplacesSymlink verifies a forced pack replaces a destination
// symlink itself, leaving the directory it pointed at untouched.
func TestPackForceReplacesSymlink(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	elsewhere := t.TempDir()
	writeFile(t, elsewhere, "keep.txt", "keep\n")
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.Symlink(elsewhere, out); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	f.pack(t, PackParams{OutDir: out, Force: true})

	if info, err := os.Lstat(out); err != nil || !info.IsDir() {
		t.Errorf("destination is not a directory: %v", err)
	}
	if got := listFiles(t, elsewhere); !slices.Equal(got, []string{"keep.txt"}) {
		t.Errorf("symlink target files = %v, want [keep.txt]", got)
	}
}

// TestPackRefusesInterruptedScratch verifies a scratch directory left by an
// interrupted pack blocks publishing rather than being overwritten.
func TestPackRefusesInterruptedScratch(t *testing.T) {
	t.Parallel()
	f := newSampleFixture(t)
	out := filepath.Join(t.TempDir(), "dist")
	writeFile(t, out, scratchName+"/previous/envx.yaml", "previous\n")

	_, err := f.service(t).Pack(PackParams{OutDir: out, Force: true})
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("err = %v, want an interrupted-pack error", err)
	}
	if got := readFile(t, out, scratchName+"/previous/envx.yaml"); got != "previous\n" {
		t.Errorf("recovery data changed to %q", got)
	}
}

// TestNewRequiresDependencies verifies construction rejects missing adapters
// and unresolved include paths.
func TestNewRequiresDependencies(t *testing.T) {
	t.Parallel()
	unresolved := workspace.Workspace{Projects: map[string]workspace.Project{
		"app": {Includes: []string{"env/app"}},
	}}
	tests := []struct {
		name string
		ws   workspace.Workspace
		deps Dependencies
	}{
		{"no store", workspace.Workspace{}, Dependencies{}},
		{"unresolved includes", unresolved, Dependencies{Secrets: newMemSecrets()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(tt.ws, tt.deps); err == nil {
				t.Error("New succeeded, want an error")
			}
		})
	}
}

// manifestFor returns a manifest declaring production and projects.
func manifestFor(projects map[string][]string) string {
	var b strings.Builder
	b.WriteString("environments: [production]\nprojects:\n")
	for _, name := range slices.Sorted(maps.Keys(projects)) {
		quoted := make([]string, len(projects[name]))
		for i, include := range projects[name] {
			quoted[i] = fmt.Sprintf("'%s'", include)
		}
		fmt.Fprintf(&b, "  %q:\n    includes: [%s]\n", name, strings.Join(quoted, ", "))
	}
	return b.String()
}
