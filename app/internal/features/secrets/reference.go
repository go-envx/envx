package secrets

import (
	"errors"
	"fmt"
	"strings"
)

// scheme is the reference prefix envx reserves for its local secrets store. Only
// values beginning with it (or its backslash-escaped form) are treated as
// references; arbitrary URL-like values such as "postgres://" pass through.
const scheme = "secret://"

// reference identifies one secret by its group and key. It is resolver
// identity, not document storage, so the document store owns no reference type.
type reference struct {
	// group is the key-group the referenced entry belongs to.
	group string
	// key is the entry's name within the group.
	key string
}

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
