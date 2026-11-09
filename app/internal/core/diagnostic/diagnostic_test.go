package diagnostic_test

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/lib/severity"
)

func TestFinding_IsZero(t *testing.T) {
	t.Parallel()

	var zero diagnostic.Finding
	if !zero.IsZero() {
		t.Errorf("zero finding IsZero() = false, want true")
	}

	cases := []struct {
		name    string
		finding diagnostic.Finding
	}{
		{
			name: "with code",
			finding: diagnostic.Finding{
				Code: diagnostic.Code("TEST_CODE"),
			},
		},
		{
			name: "with level",
			finding: diagnostic.Finding{
				Level: severity.Error,
			},
		},
		{
			name: "with project",
			finding: diagnostic.Finding{
				Project: "api",
			},
		},
		{
			name: "with environment",
			finding: diagnostic.Finding{
				Environment: "production",
			},
		},
		{
			name: "with key",
			finding: diagnostic.Finding{
				Key: "DATABASE_URL",
			},
		},
		{
			name: "with message",
			finding: diagnostic.Finding{
				Message: "something is wrong",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.finding.IsZero() {
				t.Errorf("%s: IsZero() = true, want false", tc.name)
			}
		})
	}
}
