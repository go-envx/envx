package secrets

import (
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// -------------------------------------------------------------------------------------

// Resolver dereferences secret references against a secrets store. It recognizes
// values of the form "secret://<group>/<key>"; every other value passes through
// unchanged.
type Resolver struct {
	// values holds secret values keyed by their resolver reference.
	values map[reference]string
}

// -------------------------------------------------------------------------------------

// Resolver opens the current secrets store and returns a resolver bound to it.
// A missing store yields an empty resolver, so a reference against it fails
// loudly as a dangling reference rather than leaking the raw reference string.
func (m *Manager) Resolver() (*Resolver, error) {
	// Open the current store so references resolve against its contents.
	document, err := store.Open(m.params.SecretsPath)
	if err != nil {
		return nil, err
	}
	return newResolver(document), nil
}

// -------------------------------------------------------------------------------------

// newResolver builds a resolver from a validated secrets document.
func newResolver(document *store.Document) *Resolver {
	// Index each stored secret by its normalized resolver reference.
	values := make(map[reference]string)
	for _, secret := range document.Secrets() {
		values[reference{
			group: strings.ToLower(secret.Group),
			key:   secret.Key,
		}] = secret.Value
	}
	return &Resolver{
		values: values,
	}
}

// -------------------------------------------------------------------------------------

// Resolve dereferences value against the store. A plain value is returned
// unchanged. A value beginning with the reserved scheme is parsed and looked up
// in the store, and a missing entry is an error (a dangling reference). A leading
// backslash escapes a literal that would otherwise look like a reference:
// "\secret://x" resolves to "secret://x".
func (r *Resolver) Resolve(value, _ string) (string, error) {
	// Unescape a literal value that starts with the reserved scheme.
	if strings.HasPrefix(value, `\`+scheme) {
		return value[1:], nil
	}
	// Leave ordinary values unchanged.
	if !strings.HasPrefix(value, scheme) {
		return value, nil
	}

	// Parse the reference body before looking it up.
	body := strings.TrimPrefix(value, scheme)
	ref, err := splitRef(body)
	if err != nil {
		return "", err
	}

	// Fail rather than returning an unresolved dangling reference.
	v, ok := r.values[ref]
	if !ok {
		return "", fmt.Errorf("secret %q not found in group %q", ref.key, ref.group)
	}
	return v, nil
}
