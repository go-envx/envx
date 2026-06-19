package cli

import "github.com/go-envx/envx/apps/envx/internal/shared/exitcode"

// -------------------------------------------------------------------------------------
// ExitCodeError re-exports exitcode.Error so main and tests can reference the
// type without importing the shared package directly.
type ExitCodeError = exitcode.Error
