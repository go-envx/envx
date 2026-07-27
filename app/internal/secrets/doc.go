// Package secrets resolves secret references embedded in resolved environment
// values. A reference names an entry in a local secrets store; the resolver
// looks it up and substitutes the value, leaving non-reference values untouched.
// Only values beginning with the reserved "secret://" scheme are treated as
// references, so arbitrary URL-like config values pass through unchanged.
package secrets
