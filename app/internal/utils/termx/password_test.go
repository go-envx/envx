package termx_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/utils/termx"
)

func TestPassword(t *testing.T) {
	tmpFile, err := os.Create(filepath.Join(t.TempDir(), "pwd_term"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
	})

	var out bytes.Buffer
	pwd, err := termx.Password(termx.PasswordParams{
		Out:    &out,
		In:     tmpFile,
		Prompt: "Enter password: ",
		ReadPassword: func(fd int) ([]byte, error) {
			return []byte("sample-pass-val"), nil
		},
	})
	if err != nil {
		t.Fatalf("Password() unexpected error: %v", err)
	}
	if pwd != "sample-pass-val" {
		t.Errorf("Password() = %q, want %q", pwd, "sample-pass-val")
	}
	if out.String() != "Enter password: \n" {
		t.Errorf("out = %q, want %q", out.String(), "Enter password: \n")
	}
}

func TestPasswordErrorReading(t *testing.T) {
	tmpFile, err := os.Create(filepath.Join(t.TempDir(), "pwd_term_err"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
	})

	var out bytes.Buffer
	_, err = termx.Password(termx.PasswordParams{
		Out:    &out,
		In:     tmpFile,
		Prompt: "Prompt: ",
		ReadPassword: func(fd int) ([]byte, error) {
			return nil, errors.New("read error")
		},
	})
	if err == nil {
		t.Fatal("Password() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "read error") {
		t.Errorf("expected read error, got %v", err)
	}
}
