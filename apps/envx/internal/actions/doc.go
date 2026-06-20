// Package actions holds the cobra-aware flag wiring shared by the env-resolving
// actions: it binds the engine settings flag group at the action edge so the
// engine and config packages never import cobra. Precedence resolution lives in
// the config package, not here.
package actions
