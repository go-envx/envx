package env

import (
	"testing"

	"github.com/go-envx/envx/app/internal/core/workspace"
)

// TestSet verifies include matching and environment selection.
func TestSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// include is the requested include.
		include string
		// environment is the requested environment.
		environment *string
		// wantEnvironment is the written environment.
		wantEnvironment string
		// wantErr reports an expected failure.
		wantErr bool
	}{
		{name: "relative", include: "env/app", wantEnvironment: "development"},
		{name: "yaml suffix", include: "env/app.yaml", wantEnvironment: "development"},
		{name: "absolute", include: appInclude, wantEnvironment: "development"},
		{name: "path suffix", include: "app", wantEnvironment: "development"},
		{
			name: "explicit environment", include: "env/app",
			environment: ptr("production"), wantEnvironment: "production",
		},
		{
			name: "empty environment", include: "env/app", environment: ptr(""),
			wantEnvironment: "development",
		},
		{name: "undeclared include", include: "env/other", wantErr: true},
		{
			name: "undeclared environment", include: "env/app",
			environment: ptr("staging"), wantErr: true,
		},
		{name: "empty include", include: " ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			overlays := &fakeOverlays{}
			s := newService(t, testWorkspace(), Dependencies{
				Namespaces: newFakeNamespaces(), Overlays: overlays,
			})
			got, err := s.Set(SetParams{
				Include: tt.include, Key: "db.host", Value: "x",
				Environment: tt.environment,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Set succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Set: %v", err)
			}
			if overlays.include != appInclude ||
				overlays.environment != tt.wantEnvironment ||
				overlays.key != "db.host" || overlays.text != "x" {
				t.Errorf("SetOverlay = %+v", overlays)
			}
			want := appInclude + "." + tt.wantEnvironment + ".yaml"
			if got.Key != "db.host" || got.OverlayPath != want {
				t.Errorf("Set = %+v, want path %s", got, want)
			}
		})
	}
}

// TestSetAmbiguousInclude verifies a suffix matching several includes fails
// while an exact path still selects one.
func TestSetAmbiguousInclude(t *testing.T) {
	t.Parallel()

	ws := testWorkspace()
	ws.Projects["web"] = workspace.Project{IncludePaths: []string{"/ws/web/app"}}
	overlays := &fakeOverlays{}
	s := newService(t, ws, Dependencies{
		Namespaces: newFakeNamespaces(), Overlays: overlays,
	})
	if _, err := s.Set(SetParams{Include: "app", Key: "k"}); err == nil {
		t.Error("Set with ambiguous include succeeded")
	}
	if _, err := s.Set(SetParams{Include: "web/app", Key: "k"}); err != nil ||
		overlays.include != "/ws/web/app" {
		t.Errorf("Set(web/app) wrote %q, %v", overlays.include, err)
	}
}

// TestSetRequirements verifies a writer and key are required.
func TestSetRequirements(t *testing.T) {
	t.Parallel()

	noWriter := newService(
		t, testWorkspace(), Dependencies{Namespaces: newFakeNamespaces()},
	)
	if _, err := noWriter.Set(SetParams{Include: "env/app", Key: "k"}); err == nil {
		t.Error("Set without Overlays succeeded")
	}
	s := newService(t, testWorkspace(), Dependencies{
		Namespaces: newFakeNamespaces(), Overlays: &fakeOverlays{},
	})
	if _, err := s.Set(SetParams{Include: "env/app", Key: " "}); err == nil {
		t.Error("Set with empty key succeeded")
	}
}
