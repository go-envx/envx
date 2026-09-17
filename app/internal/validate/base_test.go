package validate

import (
	"testing"

	"github.com/go-envx/envx/app/internal/envmerge"
)

// TestDeclaredInBase verifies a key is declared in base when any of its sources
// is a namespace base file, and not when it comes only from an overlay or the OS.
func TestDeclaredInBase(t *testing.T) {
	t.Parallel()

	base := envmerge.Source{File: "/ws/env/app.yaml", Key: "name"}
	overlay := envmerge.Source{File: "/ws/env/app.production.yaml", Key: "name"}
	osSource := envmerge.Source{File: "OS environment", Key: "NAME"}

	cases := []struct {
		name   string
		origin envmerge.Origin
		want   bool
	}{
		{
			name:   "base winner",
			origin: envmerge.Origin{Winner: base},
			want:   true,
		},
		{
			name:   "overlay winner shadows base",
			origin: envmerge.Origin{Winner: overlay, Shadowed: []envmerge.Source{base}},
			want:   true,
		},
		{
			name:   "overlay only",
			origin: envmerge.Origin{Winner: overlay},
			want:   false,
		},
		{
			name:   "os winner over overlay only",
			origin: envmerge.Origin{Winner: osSource, Shadowed: []envmerge.Source{overlay}},
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
