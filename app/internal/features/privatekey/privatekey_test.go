package privatekey

import (
	"errors"
	"testing"
)

// TestValidateGroup verifies accepted group names and rejected file-format
// delimiters or whitespace.
func TestValidateGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		group     string
		valid     bool
		wantErrIs error
	}{
		{name: "normal", group: "production", valid: true},
		{name: "uppercase", group: "PRODUCTION", valid: true},
		{name: "empty", group: "", valid: false, wantErrIs: ErrEmptyGroup},
		{name: "only whitespace", group: "  ", valid: false, wantErrIs: ErrEmptyGroup},
		{
			name:      "contains whitespace",
			group:     "prod uction",
			valid:     false,
			wantErrIs: ErrInvalidGroup,
		},
		{
			name:      "contains separator",
			group:     "production=value",
			valid:     false,
			wantErrIs: ErrInvalidGroup,
		},
		{
			name:      "contains line break",
			group:     "production\n",
			valid:     false,
			wantErrIs: ErrInvalidGroup,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateGroup(tt.group)
			if (err == nil) != tt.valid {
				t.Errorf("ValidateGroup(%q) error = %v, valid = %t", tt.group, err, tt.valid)
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf(
					"ValidateGroup(%q) error = %v, want errors.Is %v",
					tt.group, err, tt.wantErrIs,
				)
			}
		})
	}
}

// TestValidateEntry verifies private-key values cannot escape one-line format.
func TestValidateEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		group      string
		privateKey string
		wantErrIs  error
	}{
		{name: "valid", group: "production", privateKey: "private-value"},
		{
			name:      "empty key",
			group:     "production",
			wantErrIs: ErrEmptyKey,
		},
		{
			name:       "line feed",
			group:      "production",
			privateKey: "private\nvalue",
			wantErrIs:  ErrKeyHasLineBreak,
		},
		{
			name:       "carriage return",
			group:      "production",
			privateKey: "private\rvalue",
			wantErrIs:  ErrKeyHasLineBreak,
		},
		{
			name:       "invalid group",
			group:      "prod uction",
			privateKey: "private-value",
			wantErrIs:  ErrInvalidGroup,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEntry(tt.group, tt.privateKey)
			if tt.wantErrIs == nil && err != nil {
				t.Errorf("ValidateEntry() error = %v, want nil", err)
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("ValidateEntry() error = %v, want errors.Is %v", err, tt.wantErrIs)
			}
		})
	}
}
