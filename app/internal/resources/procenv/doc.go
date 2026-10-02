// Package procenv is the client for the process environment. It is the only
// place outside the CLI edge that reads os environment state, so services
// receive environment access through injected values rather than touching os.
package procenv
