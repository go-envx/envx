package external

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Supported reference schemes.
const (
	// SchemeOnePassword prefixes 1Password references.
	SchemeOnePassword = "op"
	// SchemeBitwarden prefixes Bitwarden Secrets Manager references.
	SchemeBitwarden = "bws"
	// SchemePasswordstate prefixes Passwordstate references.
	SchemePasswordstate = "passwordstate"
)

// schemeSeparator joins a scheme to its path.
const schemeSeparator = "://"

// schemes lists every supported scheme.
var schemes = []string{SchemeOnePassword, SchemeBitwarden, SchemePasswordstate}

// Reference is one parsed external reference.
type Reference struct {
	// Scheme names the provider, such as "op".
	Scheme string
	// Path is the provider-specific remainder after "://".
	Path string
}

// ParseReference parses a whole value as an external reference. Values without
// a supported scheme return ErrNotReference; malformed paths wrap
// ErrInvalidReference.
func ParseReference(raw string) (Reference, error) {
	scheme, ok := schemeOf(raw)
	if !ok {
		return Reference{}, ErrNotReference
	}
	path := raw[len(scheme)+len(schemeSeparator):]
	if path == "" {
		return Reference{}, fmt.Errorf("%w: empty path", ErrInvalidReference)
	}
	if strings.ContainsFunc(path, unicode.IsControl) {
		return Reference{}, fmt.Errorf(
			"%w: path contains control characters", ErrInvalidReference,
		)
	}
	if slices.Contains(strings.Split(path, "/"), "") {
		return Reference{}, fmt.Errorf("%w: empty path segment", ErrInvalidReference)
	}
	return Reference{Scheme: scheme, Path: path}, nil
}

// String returns the canonical "scheme://path" form.
func (r Reference) String() string {
	return r.Scheme + schemeSeparator + r.Path
}

// schemeOf returns raw's supported scheme, if any.
func schemeOf(raw string) (string, bool) {
	scheme, _, ok := strings.Cut(raw, schemeSeparator)
	if !ok || !slices.Contains(schemes, scheme) {
		return "", false
	}
	return scheme, true
}
