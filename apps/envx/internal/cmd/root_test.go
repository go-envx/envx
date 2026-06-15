package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommandShowsHelp(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	cmd := NewRootCmd("test")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("expected help output, got empty string")
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	cmd := NewRootCmd("1.2.3")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "envx version 1.2.3\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
