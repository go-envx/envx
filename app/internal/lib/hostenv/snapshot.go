package hostenv

import (
	"maps"
	"os"
	"strings"
)

// Snapshot represents an immutable point-in-time snapshot of environment
// variables. The zero Snapshot is empty and ready to use.
type Snapshot struct {
	values map[string]string
}

// Capture reads the active process environment via os.Environ and returns
// an immutable Snapshot.
func Capture() Snapshot {
	return From(os.Environ())
}

// From parses a slice of environment strings in "KEY=value" form into an
// immutable Snapshot. Each entry is split at the first equals sign; empty
// values are preserved. Entries without an equals sign are ignored.
// Duplicate names use the last supplied value.
func From(environ []string) Snapshot {
	if len(environ) == 0 {
		return Snapshot{}
	}
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		if name, value, found := strings.Cut(entry, "="); found {
			values[name] = value
		}
	}
	return Snapshot{values: values}
}

// Get returns the value of the named environment variable and whether it is
// present. It distinguishes an unset variable (false) from an explicitly empty
// one ("", true).
func (s Snapshot) Get(name string) (string, bool) {
	if s.values == nil {
		return "", false
	}
	value, ok := s.values[name]
	return value, ok
}

// All returns a copy of all environment variables in the snapshot as a map.
// Mutating the returned map does not affect the snapshot.
// The zero Snapshot returns an empty non-nil map.
func (s Snapshot) All() map[string]string {
	if len(s.values) == 0 {
		return make(map[string]string)
	}
	return maps.Clone(s.values)
}
