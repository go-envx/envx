// Package cli builds the root "envx" cobra command, binds the persistent
// --config flag, installs the standard streams, and registers the commands
// declared by each core/<domain>/cli package. Execute runs the tree and maps
// its outcome to the process exit code.
package cli
