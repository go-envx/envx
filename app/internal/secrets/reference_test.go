package secrets

import (
	"strings"
	"testing"
)

// TestSplitRef verifies explicit group/key parsing and canonical group names.
func TestSplitRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    reference
		wantErr string
	}{
		{
			name: "explicit group and key",
			body: "production/postgres_password",
			want: reference{group: "production", key: "postgres_password"},
		},
		{
			name: "group is case insensitive",
			body: "PRODUCTION/token",
			want: reference{group: "production", key: "token"},
		},
		{
			name:    "empty reference",
			body:    "",
			wantErr: "empty secret reference",
		},
		{
			name:    "missing key",
			body:    "production",
			wantErr: "must name a group and key",
		},
		{
			name:    "empty group",
			body:    "/token",
			wantErr: "invalid secret reference",
		},
		{
			name:    "empty key",
			body:    "production/",
			wantErr: "invalid secret reference",
		},
		{
			name:    "key contains slash",
			body:    "production/a/b",
			wantErr: "keys may not contain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := splitRef(tt.body)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("splitRef(%q) succeeded, want error", tt.body)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("splitRef(%q) error = %q, want %q", tt.body, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitRef(%q): %v", tt.body, err)
			}
			if got != tt.want {
				t.Errorf("splitRef(%q) = %+v, want %+v", tt.body, got, tt.want)
			}
		})
	}
}
