// Package core is the composition root: it holds no application logic and only
// wires the application together.
//
// Wiring flows in one direction. composeAppConfig loads the workspace manifest
// and derives each feature's Config, composeAppClients constructs every client
// from those configs, and composeAppServices constructs the domain services in
// dependency order. AppFactory memoizes one such assembly per config path and
// hands services out through accessors; workspace-free services (emit, runner,
// scaffold) never trigger a manifest load.
//
// The manifest path arrives from the caller (the CLI resolves --config and
// ENVX_CONFIG), and ENVX_* settings reach services through per-operation options,
// so core never reads process environment variables for configuration. It is
// framework-agnostic: no cobra.
package core
