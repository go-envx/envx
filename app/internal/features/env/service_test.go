package env_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/features/env"
)

type mockRepository struct {
	baseCalled    bool
	overlayCalled bool
	bases         map[string]env.NamespaceData
	overlays      map[string]map[string]env.NamespaceData
	baseErr       error
	overlayErr    error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		bases:    make(map[string]env.NamespaceData),
		overlays: make(map[string]map[string]env.NamespaceData),
	}
}

func (m *mockRepository) LoadBase(includePath string) (env.NamespaceData, error) {
	m.baseCalled = true
	if m.baseErr != nil {
		return env.NamespaceData{}, m.baseErr
	}
	if data, ok := m.bases[includePath]; ok {
		return data, nil
	}
	return env.NamespaceData{}, errors.New("base not found")
}

func (m *mockRepository) LoadOverlay(
	includePath, environment string,
) (env.NamespaceData, bool, error) {
	m.overlayCalled = true
	if m.overlayErr != nil {
		return env.NamespaceData{}, false, m.overlayErr
	}
	if envMap, ok := m.overlays[includePath]; ok {
		if data, ok := envMap[environment]; ok {
			return data, true, nil
		}
	}
	return env.NamespaceData{
		SourcePath: includePath + "." + environment + ".yaml",
	}, false, nil
}

func (m *mockRepository) SetOverlay(
	includePath, environment, key, value string,
) (string, error) {
	if m.overlayErr != nil {
		return "", m.overlayErr
	}
	return includePath + "." + environment + ".yaml", nil
}

func TestNewServiceRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := env.NewService(env.ServiceParams{
		Repository: nil,
	})
	if err == nil {
		t.Fatal("expected error when Repository is nil")
	}
}

func TestNewServicePerformsNoIO(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	_, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Includes:   []string{"missing"},
		Config:     env.Config{Environments: []string{"dev"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.baseCalled || repo.overlayCalled {
		t.Error("NewService should not perform I/O")
	}
}

func TestServiceMaterialize(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	repo.bases["app"] = env.NamespaceData{
		Data: map[string]any{
			"host": "localhost",
			"port": 5432,
		},
		SourcePath: "/fake/app.yaml",
	}
	repo.overlays["app"] = map[string]env.NamespaceData{
		"production": {
			Data: map[string]any{
				"host": "prod-db.internal",
			},
			SourcePath: "/fake/app.production.yaml",
		},
	}

	svc, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Includes:   []string{"app"},
		Config:     env.Config{Environments: []string{"development", "production"}},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	result, err := svc.Materialize(env.MaterializeParams{Environment: "production"})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if val, ok := result.Environment.Get("HOST"); !ok || val != "prod-db.internal" {
		t.Errorf("HOST = %q, want prod-db.internal", val)
	}
	if val, ok := result.Environment.Get("PORT"); !ok || val != "5432" {
		t.Errorf("PORT = %q, want 5432", val)
	}
}

func TestServiceEnvironmentNotDeclared(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	svc, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Config:     env.Config{Environments: []string{"dev"}},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Materialize(env.MaterializeParams{Environment: "prod"})
	if err == nil {
		t.Fatal("expected error for undeclared environment")
	}
	if !errors.Is(err, env.ErrEnvironmentNotDeclared) {
		t.Errorf("expected ErrEnvironmentNotDeclared, got: %v", err)
	}
}

func TestServiceRequireOverlaysMissingOverlay(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	repo.bases["app"] = env.NamespaceData{
		Data:       map[string]any{"key": "value"},
		SourcePath: "/fake/app.yaml",
	}

	svc, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Includes:   []string{"app"},
		Config: env.Config{
			Environments: []string{"dev", "prod"},
			Settings:     env.Settings{RequireOverlays: true},
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Materialize(env.MaterializeParams{Environment: "prod"})
	if err == nil {
		t.Fatal("expected error for missing required overlay")
	}
	if !errors.Is(err, env.ErrOverlayNotFound) {
		t.Errorf("expected ErrOverlayNotFound, got: %v", err)
	}
}

func TestServiceSet(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	svc, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Includes:   []string{"app"},
		Config:     env.Config{Environments: []string{"dev", "prod"}},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	result, err := svc.Set(env.SetParams{
		IncludePath: "app",
		Environment: "prod",
		Key:         "log.level",
		Value:       "debug",
	})
	if err != nil {
		t.Fatalf("svc.Set: %v", err)
	}
	if result.Key != "log.level" {
		t.Errorf("Key = %q, want log.level", result.Key)
	}
	if result.OverlayPath != "app.prod.yaml" {
		t.Errorf("OverlayPath = %q, want app.prod.yaml", result.OverlayPath)
	}
}

func TestServiceGet(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	repo.bases["app"] = env.NamespaceData{
		Data: map[string]any{
			"host": "localhost",
		},
		SourcePath: "/fake/app.yaml",
	}

	svc, err := env.NewService(env.ServiceParams{
		Repository: repo,
		Includes:   []string{"app"},
		Config:     env.Config{Environments: []string{"dev", "prod"}},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	result, err := svc.Get(env.GetParams{Key: "HOST", Environment: "dev"})
	if err != nil {
		t.Fatalf("svc.Get: %v", err)
	}
	if result.Key != "HOST" {
		t.Errorf("Key = %q, want HOST", result.Key)
	}
	if result.Value != "localhost" {
		t.Errorf("Value = %q, want localhost", result.Value)
	}
	if result.Source != "/fake/app.yaml" {
		t.Errorf("Source = %q, want /fake/app.yaml", result.Source)
	}
}
