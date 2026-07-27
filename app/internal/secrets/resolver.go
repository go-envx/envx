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

// Params is secrets' input contract: where the store lives and its group policy.
// A caller (config) populates it as plain data; Open reads the store at Path and
// returns a Resolver bound to it. Keeping it a struct mirrors envmerge.Params and
// runner.Params, so config aggregates each tool's input uniformly.
type Params struct {
	// Path is the absolute path of the secrets store (secrets.yaml).
	Path string
	// RequireGroup rejects the environment-implicit "secret://<key>" shorthand,
	// requiring every reference to name its group (secret://<group>/<key>).
	RequireGroup bool
}

// -------------------------------------------------------------------------------------

// Resolver dereferences secret references against a secrets store. It recognizes
// values of the form "secret://<group>/<key>" and, unless a group is required,
// "secret://<key>" (defaulting the group to the active environment); every other
// value passes through unchanged.
type Resolver struct {
	// store holds the secret values the resolver dereferences against.
	store *store
	// requireGroup rejects the group-less "secret://<key>" shorthand.
	requireGroup bool
}

// -------------------------------------------------------------------------------------

// Open reads the secrets store at p.Path and returns a Resolver bound to it. A
// missing store yields an empty one, so a reference against it fails loudly as a
// dangling reference rather than leaking the raw reference string. When
// p.RequireGroup is set, the environment-implicit "secret://<key>" shorthand is
// rejected.
func Open(p Params) (*Resolver, error) {
	s, err := loadStore(p.Path)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		store:        s,
		requireGroup: p.RequireGroup,
	}, nil
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

	body := strings.TrimPrefix(value, scheme)
	ref, err := r.splitRef(body, env)
	if err != nil {
		return "", err
	}

	v, ok := r.store.lookup(ref)
	if !ok {
		return "", fmt.Errorf("secret %q not found in group %q", ref.key, ref.group)
	}
	return v, nil
}

// -------------------------------------------------------------------------------------

// splitRef parses the portion of a reference after the scheme into a reference.
// "group/key" names both explicitly; a bare "key" defaults the group to the
// active environment when implicit groups are enabled. Keys may not contain "/".
func (r *Resolver) splitRef(body, env string) (reference, error) {
	parts := strings.Split(body, "/")
	switch len(parts) {
	case 1:
		return r.implicitRef(parts[0], env)
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return reference{}, fmt.Errorf("invalid secret reference %q", scheme+body)
		}
		return reference{group: parts[0], key: parts[1]}, nil
	default:
		return reference{}, fmt.Errorf(
			"invalid secret reference %q (keys may not contain '/')", scheme+body,
		)
	}
}

// -------------------------------------------------------------------------------------

// implicitRef resolves a group-less "secret://<key>" reference, defaulting the
// group to the active environment. It errors when the group is required, when no
// active environment is available, or when the key is empty.
func (r *Resolver) implicitRef(key, env string) (reference, error) {
	switch {
	case r.requireGroup:
		return reference{}, fmt.Errorf(
			"secret reference %q has no group and the shorthand is disabled",
			scheme+key,
		)
	case env == "":
		return reference{}, fmt.Errorf(
			"secret reference %q needs an active environment to resolve its group",
			scheme+key,
		)
	case key == "":
		return reference{}, errors.New("empty secret reference")
	default:
		return reference{group: env, key: key}, nil
	}
}
