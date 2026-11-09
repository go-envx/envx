package secrets

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// scheme is the reference prefix reserved for the secrets store. Only values
// beginning with it are references; other URL-like values pass through.
const scheme = "secret://"

// Reference identifies one stored secret by its group and key.
type Reference struct {
	// Group is the lowercase group name.
	Group string
	// Key is the case-sensitive key within the group.
	Key string
}

// ParseReference parses a raw value as a secret reference. Plain values and
// escaped literals ("\secret://x") return ErrNotReference; malformed
// references return a descriptive error. The group is normalized to lowercase.
// The environment is accepted for resolver parity and does not alter parsing.
func ParseReference(raw, _ string) (Reference, error) {
	body, found := strings.CutPrefix(raw, scheme)
	if !found {
		return Reference{}, ErrNotReference
	}
	group, key, found := strings.Cut(body, "/")
	if !found {
		if body == "" {
			return Reference{}, errors.New("empty secret reference")
		}
		return Reference{}, fmt.Errorf(
			"invalid secret reference %q (references must name a group and key)",
			raw,
		)
	}
	if strings.Contains(key, "/") {
		return Reference{}, fmt.Errorf(
			"invalid secret reference %q (keys may not contain '/')", raw,
		)
	}
	if group == "" || key == "" {
		return Reference{}, fmt.Errorf("invalid secret reference %q", raw)
	}
	ref, err := Reference{Group: group, Key: key}.normalized()
	if err != nil {
		return Reference{}, fmt.Errorf("invalid secret reference %q: %w", raw, err)
	}
	return ref, nil
}

// String returns the canonical reference form.
func (r Reference) String() string {
	return scheme + r.Group + "/" + r.Key
}

// normalized validates r and returns it with a lowercase group.
func (r Reference) normalized() (Reference, error) {
	group, err := NormalizeGroup(r.Group)
	if err != nil {
		return Reference{}, err
	}
	if err := ValidateKey(r.Key); err != nil {
		return Reference{}, err
	}
	return Reference{Group: group, Key: r.Key}, nil
}

// NormalizeGroup validates a group name and returns its canonical lowercase form.
func NormalizeGroup(group string) (string, error) {
	if strings.TrimSpace(group) == "" {
		return "", errors.New("secret group is empty")
	}
	if strings.IndexFunc(group, unicode.IsSpace) >= 0 ||
		strings.ContainsAny(group, "/=") || strings.HasPrefix(group, "#") {
		return "", fmt.Errorf("invalid secret group %q", group)
	}
	return strings.ToLower(group), nil
}

// ValidateKey validates a secret key identifier within a group.
func ValidateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("secret key is empty")
	}
	if strings.ContainsAny(key, "/\r\n") {
		return fmt.Errorf("invalid secret key %q", key)
	}
	return nil
}
