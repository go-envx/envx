package cmd

import "github.com/go-envx/envx/apps/envx/internal/exitcode"

// -------------------------------------------------------------------------------------
// ExitCodeError is a type alias for exitcode.Error, re-exported from the cmd
// package so that main.go and tests can reference it without importing the
// internal exitcode package directly.
type ExitCodeError = exitcode.Error
