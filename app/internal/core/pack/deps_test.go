package pack

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/secrets"
)

// memSecrets is an in-memory SecretReader.
type memSecrets struct {
	// groups holds stored groups by lowercase name.
	groups map[string]secrets.Group
	// readErr, when set, fails every Read.
	readErr error
}

// exporter records exported groups and writes "group/key=value" lines.
type exporter struct {
	// groups are the groups passed to the last export.
	groups []secrets.Group
	// err, when set, fails every export.
	err error
}

// newMemSecrets returns a reader seeded with groups.
func newMemSecrets(groups ...secrets.Group) *memSecrets {
	m := &memSecrets{groups: make(map[string]secrets.Group)}
	for _, group := range groups {
		m.groups[strings.ToLower(group.Name)] = group
	}
	return m
}

// Read returns the group matched case-insensitively.
func (m *memSecrets) Read(group string) (secrets.Snapshot, error) {
	if m.readErr != nil {
		return secrets.Snapshot{}, m.readErr
	}
	stored, ok := m.groups[strings.ToLower(group)]
	if !ok {
		return secrets.Snapshot{}, nil
	}
	return secrets.Snapshot{Group: stored, Revision: "1", Exists: true}, nil
}

// export records groups and writes one sorted line per value.
func (e *exporter) export(out io.Writer, groups []secrets.Group) error {
	if e.err != nil {
		return e.err
	}
	e.groups = groups
	for _, group := range groups {
		if group.PublicKey != "" {
			return fmt.Errorf("group %q exported with a public key", group.Name)
		}
		for _, key := range slices.Sorted(maps.Keys(group.Values)) {
			if _, err := fmt.Fprintf(
				out, "%s/%s=%s\n", group.Name, key, group.Values[key],
			); err != nil {
				return err
			}
		}
	}
	return nil
}
