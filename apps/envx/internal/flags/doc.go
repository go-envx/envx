// Package flags is the single source of truth for every envx CLI flag's identity:
// the Spec catalog (name, shorthand, ENVX_* fallback, usage, and default) that the
// config and manifest packages read for precedence resolution. It is a pure leaf
// that imports only fmt — no CLI framework. Binding these specs onto cobra
// commands lives in the config package, which owns the input boundary.
package flags
