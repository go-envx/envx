package store

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMappingEntry verifies exact and case-insensitive node lookup and reports
// absence without changing the mapping.
func TestMappingEntry(t *testing.T) {
	t.Parallel()

	mapping := newMappingNode()
	appendMappingEntry(mapping, "Production", "value")

	tests := []struct {
		name            string
		key             string
		caseInsensitive bool
		wantKey         string
		wantValue       string
		wantFound       bool
	}{
		{
			name:            "exact",
			key:             "Production",
			caseInsensitive: false,
			wantKey:         "Production",
			wantValue:       "value",
			wantFound:       true,
		},
		{
			name:            "case insensitive",
			key:             "production",
			caseInsensitive: true,
			wantKey:         "Production",
			wantValue:       "value",
			wantFound:       true,
		},
		{
			name:      "missing",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, err := getMappingEntry(
				mapping, test.key, test.caseInsensitive,
			)
			if err != nil {
				t.Fatalf("mappingEntry() error = %v", err)
			}
			if entry.found != test.wantFound {
				t.Fatalf("mappingEntry() found = %v, want %v", entry.found, test.wantFound)
			}
			if !entry.found {
				if entry.index != -1 || entry.key != "" || entry.value != nil {
					t.Fatalf("missing entry = %d, %q, %#v", entry.index, entry.key, entry.value)
				}
				return
			}
			if entry.index != 0 ||
				entry.key != test.wantKey || entry.value.Value != test.wantValue {
				t.Errorf("mappingEntry() = %d, %q, %q", entry.index, entry.key, entry.value.Value)
			}
		})
	}
}

// TestMappingEntryRejectsNonMapping verifies node lookup rejects an invalid
// YAML node instead of treating it as an empty mapping.
func TestMappingEntryRejectsNonMapping(t *testing.T) {
	t.Parallel()

	_, err := getMappingEntry(
		&yaml.Node{Kind: yaml.ScalarNode}, "key", false,
	)
	if err == nil {
		t.Error("mappingEntry() accepted a scalar node")
	}
}
