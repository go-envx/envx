package secrets

import (
	"errors"
	"fmt"
	"strings"
)

// -------------------------------------------------------------------------------------

// scheme is the reference prefix envx reserves for its local secrets store. Only
// values beginning with it (or its backslash-escaped form) are treated as
// references; arbitrary URL-like values such as "postgres://" pass through.
const scheme = "secret://"

// -------------------------------------------------------------------------------------

// Resolver dereferences secret references against a Store. It recognizes values
// of the form "secret://<group>/<key>" and, unless the group is required,
// "secret://<key>" (defaulting the group to the active environment); every other
// value passes through unchanged.
type Resolver struct {
	// store holds the secret values the resolver dereferences against.
	store *Store
	// requireGroup rejects the group-less "secret://<key>" shorthand when true.
	requireGroup bool
}

// -------------------------------------------------------------------------------------

// NewResolver builds a Resolver over store. requireGroup, when true, rejects the
// environment-implicit "secret://<key>" shorthand.
func NewResolver(store *Store, requireGroup bool) *Resolver {
	return &Resolver{store: store, requireGroup: requireGroup}
}

// -------------------------------------------------------------------------------------

// Resolve dereferences value in the context of the active environment env. A
// plain value is returned unchanged. A value beginning with the reserved scheme
// is parsed and looked up in the store, and a missing entry is an error (a
// dangling reference). A leading backslash escapes a literal that would
// otherwise look like a reference: "\secret://x" resolves to "secret://x".
func (r *Resolver) Resolve(value, env string) (string, error) {
	if strings.HasPrefix(value, `\`+scheme) {
		return value[1:], nil
	}
	if !strings.HasPrefix(value, scheme) {
		return value, nil
	}

	group, key, err := r.splitRef(strings.TrimPrefix(value, scheme), env)
	if err != nil {
		return "", err
	}

	v, ok := r.store.Lookup(group, key)
	if !ok {
		return "", fmt.Errorf("secret %q not found in group %q", key, group)
	}
	return v, nil
}

// -------------------------------------------------------------------------------------

// splitRef parses the portion of a reference after the scheme into its group and
// key. "group/key" names both explicitly; a bare "key" defaults the group to the
// active environment when implicit groups are enabled. Keys may not contain "/".
func (r *Resolver) splitRef(ref, env string) (group, key string, err error) {
	parts := strings.Split(ref, "/")
	switch len(parts) {
	case 1:
		return r.implicitRef(parts[0], env)
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid secret reference %q", scheme+ref)
		}
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf(
			"invalid secret reference %q (keys may not contain '/')", scheme+ref,
		)
	}
}

// -------------------------------------------------------------------------------------

// implicitRef resolves a group-less "secret://<key>" reference, defaulting the
// group to the active environment. It errors when the group is required, when no
// active environment is available, or when the key is empty.
func (r *Resolver) implicitRef(key, env string) (group, resolvedKey string, err error) {
	switch {
	case r.requireGroup:
		return "", "", fmt.Errorf(
			"secret reference %q has no group and the shorthand is disabled",
			scheme+key,
		)
	case env == "":
		return "", "", fmt.Errorf(
			"secret reference %q needs an active environment to resolve its group",
			scheme+key,
		)
	case key == "":
		return "", "", errors.New("empty secret reference")
	default:
		return env, key, nil
	}
}
