package secrets

import (
	"strings"
	"testing"
)

// TestNormalizeGroupName verifies canonicalization and group-name validation.
func TestNormalizeGroupName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "lowercase",
			input: "production",
			want:  "production",
		},
		{
			name:  "canonicalizes case",
			input: "Production",
			want:  "production",
		},
		{
			name:    "empty",
			wantErr: "secret group is empty",
		},
		{
			name:    "whitespace only",
			input:   " \t",
			wantErr: "secret group is empty",
		},
		{
			name:    "embedded whitespace",
			input:   "production staging",
			wantErr: "invalid secret group",
		},
		{
			name:    "slash",
			input:   "production/staging",
			wantErr: "invalid secret group",
		},
		{
			name:    "equals sign",
			input:   "production=staging",
			wantErr: "invalid secret group",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeGroupName(test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("normalizeGroupName() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeGroupName() error = %v", err)
			}
			if got != test.want {
				t.Errorf("normalizeGroupName() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestValidateSecretKey verifies key acceptance and storage-delimiter rejection.
func TestValidateSecretKey(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:  "ordinary key",
			input: "database_password",
		},
		{
			name:    "empty",
			wantErr: "secret key is empty",
		},
		{
			name:    "whitespace only",
			input:   " \t",
			wantErr: "secret key is empty",
		},
		{
			name:    "slash",
			input:   "database/password",
			wantErr: "invalid secret key",
		},
		{
			name:    "newline",
			input:   "database\npassword",
			wantErr: "invalid secret key",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateSecretKey(test.input)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateSecretKey() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("validateSecretKey() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
