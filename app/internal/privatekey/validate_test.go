package privatekey

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------

// TestValidateGroup verifies accepted group names and rejected file-format
// delimiters or whitespace.
func TestValidateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group string
		valid bool
	}{
		{name: "normal", group: "production", valid: true},
		{name: "uppercase", group: "PRODUCTION", valid: true},
		{name: "empty", group: "", valid: false},
		{name: "only whitespace", group: "  ", valid: false},
		{name: "contains whitespace", group: "prod uction", valid: false},
		{name: "contains separator", group: "production=value", valid: false},
		{name: "contains line break", group: "production\n", valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateGroup(tt.group)
			if (err == nil) != tt.valid {
				t.Errorf("validateGroup(%q) error = %v, valid = %t", tt.group, err, tt.valid)
			}
		})
	}
}

// -------------------------------------------------------------------------------------

// TestValidateEntry verifies private-key values cannot escape one-line format.
func TestValidateEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		group      string
		privateKey string
		wantError  string
	}{
		{name: "valid", group: "production", privateKey: "private-value"},
		{
			name:      "empty key",
			group:     "production",
			wantError: "private key is empty",
		},
		{
			name:       "line feed",
			group:      "production",
			privateKey: "private\nvalue",
			wantError:  "private key contains a line break",
		},
		{
			name:       "carriage return",
			group:      "production",
			privateKey: "private\rvalue",
			wantError:  "private key contains a line break",
		},
		{
			name:       "invalid group",
			group:      "prod uction",
			privateKey: "private-value",
			wantError:  "invalid private-key group",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEntry(tt.group, tt.privateKey)
			switch {
			case tt.wantError == "" && err != nil:
				t.Errorf("validateEntry() error = %v, want nil", err)
			case tt.wantError != "" &&
				(err == nil || !strings.Contains(err.Error(), tt.wantError)):
				t.Errorf("validateEntry() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}
