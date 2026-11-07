package exitcode_test

import (
	"errors"
	"io"
	"testing"

	"github.com/go-envx/envx/app/internal/lib/exitcode"
)

func TestErrorFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *exitcode.Error
		want string
	}{
		{
			name: "nil receiver",
			err:  nil,
			want: "",
		},
		{
			name: "code without cause",
			err:  &exitcode.Error{Code: 42},
			want: "exit status 42",
		},
		{
			name: "code with cause",
			err:  &exitcode.Error{Code: 1, Cause: io.EOF},
			want: "exit status 1: EOF",
		},
		{
			name: "code with wrapped error cause",
			err:  &exitcode.Error{Code: 127, Cause: errors.New("command not found")},
			want: "exit status 127: command not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	t.Parallel()

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()
		var err *exitcode.Error
		if got := err.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("nil cause", func(t *testing.T) {
		t.Parallel()
		err := &exitcode.Error{Code: 1}
		if got := err.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("non-nil cause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("underlying cause")
		err := &exitcode.Error{Code: 1, Cause: cause}
		if got := err.Unwrap(); !errors.Is(got, cause) {
			t.Errorf("Unwrap() = %v, want %v", got, cause)
		}
	})
}

func TestErrorAsType(t *testing.T) {
	t.Parallel()

	cause := errors.New("inner failure")
	exitErr := &exitcode.Error{Code: 7, Cause: cause}
	wrapped := errors.Join(errors.New("outer context"), exitErr)

	extracted, ok := errors.AsType[*exitcode.Error](wrapped)
	if !ok {
		t.Fatal("expected errors.AsType to find *exitcode.Error")
	}
	if extracted.Code != 7 {
		t.Errorf("extracted.Code = %d, want 7", extracted.Code)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("expected wrapped error chain to preserve cause via errors.Is")
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("success returns nil", func(t *testing.T) {
		t.Parallel()
		err := exitcode.New(0, nil)
		if err != nil {
			t.Errorf("New(0, nil) = %v, want nil", err)
		}
	})

	t.Run("non-zero code with nil cause", func(t *testing.T) {
		t.Parallel()
		err := exitcode.New(2, nil)
		if err == nil {
			t.Fatal("New(2, nil) returned nil, want non-nil error")
		}
		exitErr, ok := errors.AsType[*exitcode.Error](err)
		if !ok {
			t.Fatalf("expected *exitcode.Error, got %T", err)
		}
		if exitErr.Code != 2 {
			t.Errorf("Code = %d, want 2", exitErr.Code)
		}
		if exitErr.Cause != nil {
			t.Errorf("Cause = %v, want nil", exitErr.Cause)
		}
	})

	t.Run("non-zero code with cause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("process failed")
		err := exitcode.New(1, cause)
		if err == nil {
			t.Fatal("New(1, cause) returned nil, want non-nil error")
		}
		exitErr, ok := errors.AsType[*exitcode.Error](err)
		if !ok {
			t.Fatalf("expected *exitcode.Error, got %T", err)
		}
		if exitErr.Code != 1 {
			t.Errorf("Code = %d, want 1", exitErr.Code)
		}
		if !errors.Is(err, cause) {
			t.Errorf("expected error to wrap cause: %v", err)
		}
	})

	t.Run("zero code with non-nil cause", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("unexpected error")
		err := exitcode.New(0, cause)
		if err == nil {
			t.Fatal("New(0, cause) returned nil, want non-nil error")
		}
		exitErr, ok := errors.AsType[*exitcode.Error](err)
		if !ok {
			t.Fatalf("expected *exitcode.Error, got %T", err)
		}
		if exitErr.Code != 0 {
			t.Errorf("Code = %d, want 0", exitErr.Code)
		}
		if !errors.Is(err, cause) {
			t.Errorf("expected error to wrap cause: %v", err)
		}
	})
}

func TestFromCode(t *testing.T) {
	t.Parallel()

	t.Run("zero code returns nil", func(t *testing.T) {
		t.Parallel()
		err := exitcode.FromCode(0)
		if err != nil {
			t.Errorf("FromCode(0) = %v, want nil", err)
		}
	})

	t.Run("non-zero code returns error", func(t *testing.T) {
		t.Parallel()
		err := exitcode.FromCode(42)
		if err == nil {
			t.Fatal("FromCode(42) returned nil, want non-nil error")
		}
		exitErr, ok := errors.AsType[*exitcode.Error](err)
		if !ok {
			t.Fatalf("expected *exitcode.Error, got %T", err)
		}
		if exitErr.Code != 42 {
			t.Errorf("Code = %d, want 42", exitErr.Code)
		}
	})
}
