package terminal_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/terminal"
)

type failingWriter struct{}

func (f failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("failing writer error")
}

type failingReader struct{}

func (f failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("failing reader error")
}

func TestPasswordPiped(t *testing.T) {
	t.Parallel()

	t.Run("reads line with trailing newline", func(t *testing.T) {
		var out bytes.Buffer
		in := strings.NewReader("my-secret-password\n")

		pwd, err := terminal.Password(in, &out, "Password: ")
		if err != nil {
			t.Fatalf("Password() error = %v", err)
		}
		if pwd != "my-secret-password" {
			t.Errorf("Password() = %q, want %q", pwd, "my-secret-password")
		}
		if out.String() != "Password: \n" {
			t.Errorf("out = %q, want %q", out.String(), "Password: \n")
		}
	})

	t.Run("reads line with CRLF", func(t *testing.T) {
		var out bytes.Buffer
		in := strings.NewReader("crlf-password\r\n")

		pwd, err := terminal.Password(in, &out, "")
		if err != nil {
			t.Fatalf("Password() error = %v", err)
		}
		if pwd != "crlf-password" {
			t.Errorf("Password() = %q, want %q", pwd, "crlf-password")
		}
		if out.Len() != 0 {
			t.Errorf("out = %q, want empty", out.String())
		}
	})

	t.Run("reads line ending at EOF", func(t *testing.T) {
		var out bytes.Buffer
		in := strings.NewReader("eof-password")

		pwd, err := terminal.Password(in, &out, "Key: ")
		if err != nil {
			t.Fatalf("Password() error = %v", err)
		}
		if pwd != "eof-password" {
			t.Errorf("Password() = %q, want %q", pwd, "eof-password")
		}
		if out.String() != "Key: \n" {
			t.Errorf("out = %q, want %q", out.String(), "Key: \n")
		}
	})

	t.Run("empty input returns EOF", func(t *testing.T) {
		var out bytes.Buffer
		in := strings.NewReader("")

		_, err := terminal.Password(in, &out, "Key: ")
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Password() error = %v, want EOF", err)
		}
	})

	t.Run("nil input returns EOF", func(t *testing.T) {
		var out bytes.Buffer
		_, err := terminal.Password(nil, &out, "Key: ")
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Password() error = %v, want EOF", err)
		}
	})

	t.Run("nil output with valid input", func(t *testing.T) {
		in := strings.NewReader("secret-no-out\n")
		pwd, err := terminal.Password(in, nil, "Prompt: ")
		if err != nil {
			t.Fatalf("Password() error = %v", err)
		}
		if pwd != "secret-no-out" {
			t.Errorf("Password() = %q, want %q", pwd, "secret-no-out")
		}
	})

	t.Run("error writing prompt", func(t *testing.T) {
		in := strings.NewReader("secret\n")
		_, err := terminal.Password(in, failingWriter{}, "Prompt: ")
		if err == nil {
			t.Fatal("Password() expected error when prompt write fails, got nil")
		}
		if !strings.Contains(err.Error(), "writing prompt") {
			t.Errorf("error = %v, want writing prompt error", err)
		}
	})

	t.Run("error reading from stream", func(t *testing.T) {
		var out bytes.Buffer
		_, err := terminal.Password(failingReader{}, &out, "Prompt: ")
		if err == nil {
			t.Fatal("Password() expected error when stream read fails, got nil")
		}
		if !strings.Contains(err.Error(), "reading password") {
			t.Errorf("error = %v, want reading password error", err)
		}
	})
}

func TestPasswordTerminal(t *testing.T) {
	cleanupTerm := terminal.ExportSetIsTerminalFD(func(fd int) bool {
		return isTerminalFDMock != nil && isTerminalFDMock(fd)
	})
	cleanupPwd := terminal.ExportSetReadPassword(func(fd int) ([]byte, error) {
		if readPasswordMock != nil {
			return readPasswordMock(fd)
		}
		return nil, nil
	})
	t.Cleanup(func() {
		cleanupTerm()
		cleanupPwd()
	})

	tmpFile, err := os.Create(filepath.Join(t.TempDir(), "pwd_mock_term"))
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
	})

	isTerminalFDMock = func(fd int) bool {
		return fd == int(tmpFile.Fd())
	}

	t.Run("success", func(t *testing.T) {
		readPasswordMock = func(fd int) ([]byte, error) {
			return []byte("term-secret-val"), nil
		}

		var out bytes.Buffer
		pwd, err := terminal.Password(tmpFile, &out, "Enter terminal password: ")
		if err != nil {
			t.Fatalf("Password() error = %v", err)
		}
		if pwd != "term-secret-val" {
			t.Errorf("Password() = %q, want %q", pwd, "term-secret-val")
		}
		if out.String() != "Enter terminal password: \n" {
			t.Errorf("out = %q, want %q", out.String(), "Enter terminal password: \n")
		}
	})

	t.Run("read error", func(t *testing.T) {
		readPasswordMock = func(fd int) ([]byte, error) {
			return nil, errors.New("simulated term read error")
		}

		var out bytes.Buffer
		_, err := terminal.Password(tmpFile, &out, "Prompt: ")
		if err == nil {
			t.Fatal("Password() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "simulated term read error") {
			t.Errorf("error = %v, want simulated term read error", err)
		}
	})
}

var (
	isTerminalFDMock func(int) bool
	readPasswordMock func(int) ([]byte, error)
)
