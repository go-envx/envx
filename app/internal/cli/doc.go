// Package cli builds the root "envx" cobra command and registers each action.
// It owns the persistent --config flag and forwards its value (a path string) to
// every subcommand, which loads and resolves the manifest on demand.
package cli
