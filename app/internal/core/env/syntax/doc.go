// Package syntax owns the variable reference grammar: tokenizing values into
// literal text and references, and substituting references through a caller
// lookup. It knows nothing about workspaces, dependency graphs, or secrets.
package syntax
