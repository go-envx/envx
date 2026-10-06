// Package hostenv provides a snapshot client for the host process environment.
// It is the only place outside the CLI edge that reads OS environment state,
// allowing services to receive environment access through injected values rather
// than touching os directly.
package hostenv
