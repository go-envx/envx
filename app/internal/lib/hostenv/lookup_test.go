package hostenv_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

func writeNamed(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFromParsesEnviron(t *testing.T) {
	t.Parallel()

	l := hostenv.From([]string{"A=1", "EMPTY=", "URL=x?a=b", "NOEQ", "A=2"})
	cases := map[string]string{"A": "2", "EMPTY": "", "URL": "x?a=b"}
	for name, want := range cases {
		v, ok, err := l.Get(name)
		if err != nil || !ok || v.Value != want || v.File != "" {
			t.Errorf("Get(%q) = (%+v, %v, %v), want value %q", name, v, ok, err, want)
		}
	}
	if _, ok, _ := l.Get("NOEQ"); ok {
		t.Error("entry without '=' should be ignored")
	}
}

func TestNilLookupIsEmpty(t *testing.T) {
	t.Parallel()

	var l *hostenv.Lookup
	if _, ok, err := l.Get("X"); ok || err != nil {
		t.Errorf("Get = (%v, %v), want absent", ok, err)
	}
	all, err := l.All()
	if err != nil || all == nil || len(all) != 0 {
		t.Errorf("All = (%v, %v), want empty non-nil map", all, err)
	}
}

func TestDefaultEnvironUsesProcess(t *testing.T) {
	t.Setenv("LIB_HOSTENV_PRESENT", "yes")

	v, ok, err := hostenv.NewLookup(hostenv.Options{}).Get("LIB_HOSTENV_PRESENT")
	if err != nil || !ok || v.Value != "yes" {
		t.Errorf("Get = (%+v, %v, %v)", v, ok, err)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	t.Parallel()

	l := hostenv.From([]string{"A=1"})
	all, _ := l.All()
	all["A"] = hostenv.Var{Value: "changed"}
	delete(all, "A")
	if v, ok, _ := l.Get("A"); !ok || v.Value != "1" {
		t.Errorf("lookup was mutated: (%+v, %v)", v, ok)
	}
}

func TestValuesReturnsValuesByName(t *testing.T) {
	t.Parallel()

	got, err := hostenv.From([]string{"A=1", "EMPTY="}).Values()
	if err != nil || len(got) != 2 || got["A"] != "1" || got["EMPTY"] != "" {
		t.Errorf("Values() = %v, %v", got, err)
	}
	var nilLookup *hostenv.Lookup
	if got, err := nilLookup.Values(); err != nil || len(got) != 0 {
		t.Errorf("nil Values() = %v, %v", got, err)
	}
}

func TestPrecedence(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"envx.local.yaml", ".env"} {
		content := "BOTH: file\nONLY_FILE: f\nEMPTY_WINS: file\n"
		if name == ".env" {
			content = "BOTH=file\nONLY_FILE=f\nEMPTY_WINS=file\n"
		}
		file := writeNamed(t, name, content)
		l := hostenv.NewLookup(hostenv.Options{
			File:    file,
			Environ: func() []string { return []string{"BOTH=proc", "EMPTY_WINS="} },
		})

		tests := []struct {
			name, value, file string
		}{
			{"BOTH", "proc", ""},
			{"ONLY_FILE", "f", file},
			{"EMPTY_WINS", "", ""},
		}
		for _, tc := range tests {
			v, ok, err := l.Get(tc.name)
			if err != nil || !ok || v.Value != tc.value || v.File != tc.file {
				t.Errorf(
					"%s: Get(%q) = (%+v, %v, %v), want (%q, %q)",
					name, tc.name, v, ok, err, tc.value, tc.file,
				)
			}
		}
	}
}

func TestMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	l := hostenv.NewLookup(hostenv.Options{
		File:    filepath.Join(t.TempDir(), "absent.yaml"),
		Environ: func() []string { return []string{"A=1"} },
	})
	all, err := l.All()
	if err != nil || len(all) != 1 {
		t.Errorf("All = (%v, %v), want one variable", all, err)
	}
}

func TestUnreadableFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l := hostenv.NewLookup(hostenv.Options{
		File:    dir,
		Environ: func() []string { return nil },
	})
	_, err := l.All()
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Errorf("err = %v, want one naming %s", err, dir)
	}
}

func TestParseDotenv(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"# comment",
		"",
		"PLAIN=value",
		"export EXPORTED=yes",
		"SINGLE='lit $$X # kept'",
		`DOUBLE="a\nb"`,
		"EMPTY=",
		"EQ=a=b",
		"DUP=first",
		"DUP=last",
	}, "\n")
	l := hostenv.NewLookup(hostenv.Options{
		File:    writeNamed(t, ".env", content),
		Environ: func() []string { return nil },
	})
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "yes",
		"SINGLE":   "lit $$X # kept",
		"DOUBLE":   "a\nb",
		"EMPTY":    "",
		"EQ":       "a=b",
		"DUP":      "last",
	}
	all, err := l.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(want) {
		t.Errorf("got %d variables, want %d: %v", len(all), len(want), all)
	}
	for name, value := range want {
		if got := all[name].Value; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestParseYAML(t *testing.T) {
	t.Parallel()

	content := "# comment\nPLAIN: value\nDOLLAR: pa$$word\n" +
		"QUOTED: \"1234\"\nEMPTY: \"\"\n"
	want := map[string]string{
		"PLAIN": "value", "DOLLAR": "pa$$word", "QUOTED": "1234", "EMPTY": "",
	}
	for _, name := range []string{"envx.local.yaml", "envx.local.yml", "A.YAML"} {
		l := hostenv.NewLookup(hostenv.Options{
			File:    writeNamed(t, name, content),
			Environ: func() []string { return nil },
		})
		all, err := l.All()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(all) != len(want) {
			t.Errorf("%s: got %v, want %v", name, all, want)
		}
		for key, value := range want {
			if got := all[key].Value; got != value {
				t.Errorf("%s: %s = %q, want %q", name, key, got, value)
			}
		}
	}
}

func TestEmptyYAMLFile(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", "# only a comment\n", "---\n", "null\n"} {
		l := hostenv.NewLookup(hostenv.Options{
			File:    writeNamed(t, "envx.local.yaml", content),
			Environ: func() []string { return nil },
		})
		if all, err := l.All(); err != nil || len(all) != 0 {
			t.Errorf("%q: All = (%v, %v), want empty", content, all, err)
		}
	}
}

func TestYAMLErrorsNameFileAndLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, want string
	}{
		{"not a mapping", "- a\n- b\n", ":1:"},
		{"nested value", "A: 1\nB:\n  C: x\n", ":1:"},
		{"number", "PORT: 5432\n", ":1:"},
		{"bool", "A: x\nFLAG: true\n", ":2:"},
		{"null value", "A:\n", ":1:"},
		{"alias", "A: &a x\nB: *a\n", ":2:"},
		{"duplicate", "A: x\nB: y\nA: z\n", ":3:"},
		{"bad name", "\n\"1BAD\": x\n", ":2:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			file := writeNamed(t, "envx.local.yaml", tc.content)
			_, err := hostenv.NewLookup(hostenv.Options{File: file}).All()
			if err == nil || !strings.Contains(err.Error(), file+tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, file+tc.want)
			}
		})
	}
}

func TestMalformedFilesNameFile(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		".env":            "A=\"unterminated\n",
		"envx.local.yaml": "A: [unclosed\n",
	} {
		file := writeNamed(t, name, content)
		_, err := hostenv.NewLookup(hostenv.Options{File: file}).All()
		if err == nil || !strings.Contains(err.Error(), file) {
			t.Errorf("%s: err = %v, want one naming the file", name, err)
		}
	}
}

func TestReadsOnceAndMemoizesErrors(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	l := hostenv.NewLookup(hostenv.Options{Environ: func() []string {
		calls.Add(1)
		return []string{"A=1"}
	}})
	for range 3 {
		if _, _, err := l.Get("A"); err != nil {
			t.Fatal(err)
		}
		if _, err := l.All(); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("environment read %d times, want 1", got)
	}

	bad := hostenv.NewLookup(hostenv.Options{
		File: writeNamed(t, "envx.local.yaml", "BROKEN\n"),
	})
	_, _, first := bad.Get("A")
	_, second := bad.All()
	if first == nil || second == nil || first.Error() != second.Error() {
		t.Errorf("errors = %v, %v, want the same memoized error", first, second)
	}
}

func TestConcurrentFirstReads(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	l := hostenv.NewLookup(hostenv.Options{Environ: func() []string {
		calls.Add(1)
		return []string{"A=1"}
	}})
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if v, ok, err := l.Get("A"); err != nil || !ok || v.Value != "1" {
				t.Errorf("Get = (%+v, %v, %v)", v, ok, err)
			}
		})
	}
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Errorf("environment read %d times, want 1", got)
	}
}
