package hostenv

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"strings"
	"sync"
)

// Options configures a Lookup.
type Options struct {
	// File is an optional variables file; .yaml and .yml are read as a flat
	// YAML mapping and anything else as dotenv. Empty reads none.
	File string
	// Environ returns the process environment as KEY=value entries; nil uses
	// os.Environ.
	Environ func() []string
}

// Var is one host variable and where it came from.
type Var struct {
	Value string
	// File is the dotenv file that supplied the value; empty for the process
	// environment.
	File string
}

// Lookup serves host variables from the process environment and an optional
// variables file. The process environment wins. It reads both once, on first use,
// and is safe for concurrent use. A nil *Lookup is empty.
type Lookup struct {
	load func() (map[string]Var, error)
}

// NewLookup returns a Lookup that defers all reads to its first use.
func NewLookup(opts Options) *Lookup {
	return &Lookup{load: sync.OnceValues(func() (map[string]Var, error) {
		return read(opts)
	})}
}

// From returns a Lookup over a fixed environment, for tests.
func From(environ []string) *Lookup {
	return NewLookup(Options{Environ: func() []string { return environ }})
}

// Get returns the named variable and whether it is present. An empty value is
// present.
func (l *Lookup) Get(name string) (Var, bool, error) {
	if l == nil {
		return Var{}, false, nil
	}
	vars, err := l.load()
	if err != nil {
		return Var{}, false, err
	}
	v, ok := vars[name]
	return v, ok, nil
}

// All returns a copy of every variable.
func (l *Lookup) All() (map[string]Var, error) {
	if l == nil {
		return map[string]Var{}, nil
	}
	vars, err := l.load()
	if err != nil {
		return nil, err
	}
	return maps.Clone(vars), nil
}

// Values returns every variable's value by name.
func (l *Lookup) Values() (map[string]string, error) {
	vars, err := l.All()
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(vars))
	for name, v := range vars {
		values[name] = v.Value
	}
	return values, nil
}

// read merges the file under the process environment.
func read(opts Options) (map[string]Var, error) {
	environ := os.Environ
	if opts.Environ != nil {
		environ = opts.Environ
	}
	vars := make(map[string]Var)
	if opts.File != "" {
		fileVars, err := readFile(opts.File)
		if err != nil {
			return nil, err
		}
		for name, value := range fileVars {
			vars[name] = Var{Value: value, File: opts.File}
		}
	}
	for _, entry := range environ() {
		if name, value, found := strings.Cut(entry, "="); found {
			vars[name] = Var{Value: value}
		}
	}
	return vars, nil
}

// readFile parses a variables file; a missing file is empty.
func readFile(path string) (map[string]string, error) {
	//nolint:gosec // the path is the configured local env file
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	values, err := parseFile(path, data)
	if err != nil {
		return nil, fileError(path, err)
	}
	return values, nil
}
