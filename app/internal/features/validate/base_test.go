package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
)

// TestDeclaredInBase verifies a key is declared in base when any of its sources
// is a namespace base file, and not when it comes only from an overlay or the OS.
func TestDeclaredInBase(t *testing.T) {
	t.Parallel()

	base := env.Source{File: "/ws/env/app.yaml", Key: "name"}
	overlay := env.Source{File: "/ws/env/app.production.yaml", Key: "name"}
	osSource := env.Source{File: "OS environment", Key: "NAME"}

	cases := []struct {
		name   string
		origin env.Origin
		want   bool
	}{
		{
			name:   "base winner",
			origin: env.Origin{Winner: base},
			want:   true,
		},
		{
			name:   "overlay winner shadows base",
			origin: env.Origin{Winner: overlay, Shadowed: []env.Source{base}},
			want:   true,
		},
		{
			name:   "overlay only",
			origin: env.Origin{Winner: overlay},
			want:   false,
		},
		{
			name:   "os winner over overlay only",
			origin: env.Origin{Winner: osSource, Shadowed: []env.Source{overlay}},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := declaredInBase(tc.origin, "production"); got != tc.want {
				t.Errorf("declaredInBase() = %v, want %v", got, tc.want)
			}
		})
	}
}
