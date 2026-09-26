package workspace_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/features/workspace"
)

// fakeRepository implements workspace.Repository for fast unit testing.
type fakeRepository struct {
	workspace *workspace.Workspace
	err       error
}

func (f *fakeRepository) Load() (*workspace.Workspace, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.workspace, nil
}

// TestServiceLoad verifies Service.Load delegates to the injected repository.
func TestServiceLoad(t *testing.T) {
	t.Parallel()

	expectedWS := &workspace.Workspace{
		Path:         "/fake/envx.yaml",
		Root:         "/fake",
		Environments: []string{"dev", "prod"},
	}

	repo := &fakeRepository{workspace: expectedWS}
	svc, err := workspace.NewService(workspace.ServiceParams{Repository: repo})
	if err != nil {
		t.Fatalf("NewService() unexpected error: %v", err)
	}

	got, err := svc.Load()
	if err != nil {
		t.Fatalf("svc.Load() unexpected error: %v", err)
	}
	if got != expectedWS {
		t.Errorf("svc.Load() = %v, want %v", got, expectedWS)
	}
}

// TestServiceLoadError verifies Service.Load propagates errors from the repository.
func TestServiceLoadError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("storage error")
	repo := &fakeRepository{err: expectedErr}
	svc, err := workspace.NewService(workspace.ServiceParams{Repository: repo})
	if err != nil {
		t.Fatalf("NewService() unexpected error: %v", err)
	}

	_, err = svc.Load()
	if !errors.Is(err, expectedErr) {
		t.Errorf("svc.Load() err = %v, want %v", err, expectedErr)
	}
}

// TestNewServiceRequiresRepository verifies NewService rejects a nil Repository.
func TestNewServiceRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := workspace.NewService(workspace.ServiceParams{})
	if err == nil {
		t.Fatal("expected error when repository is nil")
	}
}
