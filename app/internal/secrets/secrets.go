package secrets

import (
	"errors"
	"fmt"
	"strings"

	secretstore "github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// -------------------------------------------------------------------------------------

// scheme is the reference prefix envx reserves for its local secrets store. Only
// values beginning with it (or its backslash-escaped form) are treated as
// references; arbitrary URL-like values such as "postgres://" pass through.
const scheme = "secret://"

// -------------------------------------------------------------------------------------

// Params is secrets' input contract: where the store and private-key file live.
// A caller (config) populates it as plain data; Open reads the store at
// SecretsPath and returns a Resolver bound to it.
type Params struct {
	// SecretsPath is the absolute path of the secrets store (secrets.yaml).
	SecretsPath string
	// KeysPath is the absolute path of the private-key file (envx.keys).
	KeysPath string
}

// -------------------------------------------------------------------------------------

// reference identifies one secret by its group and key. It is resolver
// identity, not document storage, so the document store owns no reference type.
type reference struct {
	// group is the key-group the referenced entry belongs to.
	group string
	// key is the entry's name within the group.
	key string
}

// -------------------------------------------------------------------------------------

// Resolver dereferences secret references against a secrets store. It recognizes
// values of the form "secret://<group>/<key>"; every other value passes through
// unchanged.
type Resolver struct {
	// values holds secret values keyed by their resolver reference.
	values map[reference]string
}

// -------------------------------------------------------------------------------------

// Open reads the secrets store at p.SecretsPath and returns a Resolver bound to
// it. A missing store yields an empty one, so a reference against it fails loudly
// as a dangling reference rather than leaking the raw reference string.
func Open(p Params) (*Resolver, error) {
	document, err := secretstore.Open(p.SecretsPath)
	if err != nil {
		return nil, err
	}
	values := make(map[reference]string)
	for _, secret := range document.Secrets() {
		values[reference{
			group: strings.ToLower(secret.Group),
			key:   secret.Key,
		}] = secret.Value
	}
	return &Resolver{
		values: values,
	}, nil
}

// -------------------------------------------------------------------------------------

// Resolve dereferences value against the store. A plain value is returned
// unchanged. A value beginning with the reserved scheme is parsed and looked up
// in the store, and a missing entry is an error (a dangling reference). A leading
// backslash escapes a literal that would otherwise look like a reference:
// "\secret://x" resolves to "secret://x".
func (r *Resolver) Resolve(value, _ string) (string, error) {
	if strings.HasPrefix(value, `\`+scheme) {
		return value[1:], nil
	}
	if !strings.HasPrefix(value, scheme) {
		return value, nil
	}

	body := strings.TrimPrefix(value, scheme)
	ref, err := splitRef(body)
	if err != nil {
		return "", err
	}

	v, ok := r.values[ref]
	if !ok {
		return "", fmt.Errorf("secret %q not found in group %q", ref.key, ref.group)
	}
	return v, nil
}

// -------------------------------------------------------------------------------------

// splitRef parses the portion of a reference after the scheme into a reference.
// References must name both the group and key explicitly. Keys may not contain
// "/".
func splitRef(body string) (reference, error) {
	group, key, found := strings.Cut(body, "/")
	if !found {
		if body == "" {
			return reference{}, errors.New("empty secret reference")
		}
		return reference{}, fmt.Errorf(
			"invalid secret reference %q (references must name a group and key)",
			scheme+body,
		)
	}
	if strings.Contains(key, "/") {
		return reference{}, fmt.Errorf(
			"invalid secret reference %q (keys may not contain '/')", scheme+body,
		)
	}
	if group == "" || key == "" {
		return reference{}, fmt.Errorf("invalid secret reference %q", scheme+body)
	}
	return reference{group: strings.ToLower(group), key: key}, nil
}
