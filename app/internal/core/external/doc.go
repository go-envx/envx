// Package external resolves references to external secret managers, such as
// "op://vault/item/field", for the providers a workspace enables.
//
// Resolver decorates the store resolver: references to enabled providers are
// masked or fetched, while every other value goes to the wrapped resolver.
// Fetches run one at a time on demand and are cached, including failures, for
// the resolver's lifetime.
package external
