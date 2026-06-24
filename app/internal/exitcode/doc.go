// Package exitcode provides a shared error type for propagating a numeric exit
// code through the application up to main.go. It exists as its own leaf package
// so any layer can reference the type without creating an import cycle.
package exitcode
