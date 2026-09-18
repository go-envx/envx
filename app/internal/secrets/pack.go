package secrets

import (
	"strings"

	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// packStoreIndent is the block indentation applied to a filtered bundle store
// that has no indentation of its own to preserve.
const packStoreIndent = 2

// WriteFilteredStore reads the secrets store at src, keeps only the values named
// by referenced, drops every public key, and writes the result to dst. It is the
// store side of `envx pack`: a bundle carries only the secrets its selected
// environments reference and, being run-only, no encryption keys. It performs no
// decryption — ciphertext travels through unchanged. Groups are matched
// case-insensitively, mirroring reference resolution.
func WriteFilteredStore(src, dst string, referenced []SecretReference) error {
	document, err := store.Open(src)
	if err != nil {
		return err
	}

	keep := make(map[reference]bool, len(referenced))
	for _, r := range referenced {
		keep[reference{group: strings.ToLower(r.Group), key: r.Key}] = true
	}
	retain := func(group, key string) bool {
		return keep[reference{group: strings.ToLower(group), key: key}]
	}

	if err := document.RetainSecrets(retain); err != nil {
		return err
	}
	if err := document.RemovePublicKeys(); err != nil {
		return err
	}
	return document.SaveTo(dst, packStoreIndent)
}
