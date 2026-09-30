package secrets

// StoredSecrets returns every stored entry's identity and encryption state in
// document order, reading the store without decrypting or exposing any value. An
// encrypted value's envelope algorithm is compared to the configured cipher so a
// value stored under a different algorithm is surfaced. A missing store yields an
// empty slice rather than an error, so a workspace with no secrets validates
// cleanly.
func (s *Service) StoredSecrets() ([]StoredSecret, error) {
	stored, err := s.params.Repository.ListSecrets()
	if err != nil {
		return nil, err
	}

	configured := s.params.Cipher.Algorithm()
	out := make([]StoredSecret, 0, len(stored))
	for _, secret := range stored {
		entry := StoredSecret{
			Group:     secret.Group,
			Key:       secret.Key,
			Encrypted: s.codec.isCiphertext(secret.Ciphertext),
		}
		// Compare a decodable envelope's algorithm to the configured cipher. A value
		// that is not ciphertext, or a malformed envelope, is left for the encryption
		// check; only a well-formed envelope reports an algorithm mismatch.
		if entry.Encrypted {
			if algorithm, _, decodeErr := s.codec.decode(secret.Ciphertext); decodeErr == nil {
				entry.AlgorithmMismatch = algorithm != configured
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// GroupsMissingPublicKey returns every group that has stored secrets but no
// stored public key, in first-seen document order. Such a group's values can
// never be decrypted because there is no key to have encrypted them under, so it
// is a store-health problem no single reference reveals. A missing store yields
// an empty slice rather than an error.
func (s *Service) GroupsMissingPublicKey() ([]string, error) {
	stored, err := s.params.Repository.ListSecrets()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, secret := range stored {
		if seen[secret.Group] {
			continue
		}
		seen[secret.Group] = true
		if _, ok, err := s.params.Repository.GetPublicKey(secret.Group); err != nil {
			return nil, err
		} else if !ok {
			out = append(out, secret.Group)
		}
	}
	return out, nil
}
