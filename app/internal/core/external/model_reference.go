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

// fieldSeparator joins a path to its field fragment.
const fieldSeparator = "#"

// schemes lists every supported scheme.
var schemes = []string{SchemeOnePassword, SchemeBitwarden, SchemePasswordstate}

// Reference is one parsed external reference.
type Reference struct {
	// Scheme names the provider, such as "op".
	Scheme string
	// Path is the provider-specific remainder after "://", without the field.
	Path string
	// Field is the "#" fragment naming a record field; empty when absent.
	Field string
}

// ParseReference parses a whole value as an external reference. Values without
// a supported scheme return ErrNotReference; malformed paths wrap
// ErrInvalidReference.
func ParseReference(raw string) (Reference, error) {
	scheme, ok := SchemeOf(raw)
	if !ok {
		return Reference{}, ErrNotReference
	}
	rest := raw[len(scheme)+len(schemeSeparator):]
	if strings.ContainsFunc(rest, unicode.IsControl) {
		return Reference{}, fmt.Errorf(
			"%w: path contains control characters", ErrInvalidReference,
		)
	}
	path, field, hasField := strings.Cut(rest, fieldSeparator)
	switch {
	case path == "":
		return Reference{}, fmt.Errorf("%w: empty path", ErrInvalidReference)
	case slices.Contains(strings.Split(path, "/"), ""):
		return Reference{}, fmt.Errorf("%w: empty path segment", ErrInvalidReference)
	case hasField && field == "":
		return Reference{}, fmt.Errorf("%w: empty field", ErrInvalidReference)
	case strings.Contains(field, fieldSeparator):
		return Reference{}, fmt.Errorf(
			"%w: more than one %q; percent-encode a literal one as %%23",
			ErrInvalidReference, fieldSeparator,
		)
	}
	return Reference{Scheme: scheme, Path: path, Field: field}, nil
}

// String returns the canonical "scheme://path[#field]" form.
func (r Reference) String() string {
	if r.Field == "" {
		return r.Scheme + schemeSeparator + r.Path
	}
	return r.Scheme + schemeSeparator + r.Path + fieldSeparator + r.Field
}

// SchemeOf returns raw's supported scheme, if any, without checking its path.
// An escaped value has no scheme.
func SchemeOf(raw string) (string, bool) {
	scheme, _, ok := strings.Cut(raw, schemeSeparator)
	if !ok || !slices.Contains(schemes, scheme) {
		return "", false
	}
	return scheme, true
}
