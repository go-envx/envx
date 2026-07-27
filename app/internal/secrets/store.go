package secrets

// -------------------------------------------------------------------------------------

// store is the parsed secrets file: secret values keyed by the reference that
// locates them. In this phase values are stored verbatim (plaintext); a later
// phase will store ciphertext and decrypt on lookup, without changing this
// type's shape. It is unexported: callers work through Resolver, never the store
// directly.
type store struct {
	// secrets maps each reference to its secret value.
	secrets map[reference]string
}

// -------------------------------------------------------------------------------------

// reference is a parsed secret reference: the group and key that together locate
// one entry in the secrets store. splitRef produces it from the text after the
// scheme, and Resolve looks it up. Bundling the pair keeps the two identifiers
// (which only mean anything together) from being threaded around as loose return
// values. It names where a value lives, not how it is stored — encryption
// metadata belongs to the store entry, decrypted on lookup, not here.
type reference struct {
	// group is the key-group the referenced entry belongs to.
	group string
	// key is the entry's name within the group.
	key string
}

// -------------------------------------------------------------------------------------

// lookup returns the value of ref and whether it was present.
func (s *store) lookup(ref reference) (string, bool) {
	v, ok := s.secrets[ref]
	return v, ok
}
