package secrets

import "strings"

// StoredSecret reports a stored value's encoding without the value itself.
type StoredSecret struct {
	// Reference identifies the stored value.
	Reference Reference
	// Encrypted reports whether the value claims the envelope format.
	Encrypted bool
	// AlgorithmMismatch reports a well-formed envelope from another cipher.
	AlgorithmMismatch bool
}

// AuditScope selects audit sections; the zero scope selects all of them.
type AuditScope struct {
	// Secrets reports each stored value's encoding.
	Secrets bool
	// Keypairs reports private-key status for groups with a public key.
	Keypairs bool
	// MissingPublicKeys reports groups that store values without a public key.
	MissingPublicKeys bool
}

// normalized expands the zero scope to every section.
func (s AuditScope) normalized() AuditScope {
	if s == (AuditScope{}) {
		return AuditScope{Secrets: true, Keypairs: true, MissingPublicKeys: true}
	}
	return s
}

// Audit is a store health report containing no secret values or private keys.
type Audit struct {
	// Secrets lists stored values in group order, then key order.
	Secrets []StoredSecret
	// Keypairs lists the key status of each group with a public key.
	Keypairs []KeypairMetadata
	// GroupsMissingPublicKey lists groups that store values without a public key.
	GroupsMissingPublicKey []string
}

// Audit reports store health for the selected sections without decrypting.
// Only the Keypairs section resolves private keys.
func (s *Service) Audit(scope AuditScope) (Audit, error) {
	scope = scope.normalized()
	snaps, err := s.store.List()
	if err != nil {
		return Audit{}, err
	}
	audit := Audit{
		Secrets:                []StoredSecret{},
		Keypairs:               []KeypairMetadata{},
		GroupsMissingPublicKey: []string{},
	}
	for _, snap := range snaps {
		group := strings.ToLower(snap.Group.Name)
		if scope.Secrets {
			for _, key := range s.sortedKeys(snap.Group.Values) {
				audit.Secrets = append(audit.Secrets, s.storedSecret(
					Reference{Group: group, Key: key}, snap.Group.Values[key],
				))
			}
		}
		if scope.Keypairs && snap.Group.PublicKey != "" {
			metadata, err := s.inspect(group, snap.Group.PublicKey)
			if err != nil {
				return Audit{}, err
			}
			audit.Keypairs = append(audit.Keypairs, metadata)
		}
		if scope.MissingPublicKeys && snap.Group.PublicKey == "" &&
			len(snap.Group.Values) > 0 {
			audit.GroupsMissingPublicKey = append(audit.GroupsMissingPublicKey, group)
		}
	}
	return audit, nil
}

// storedSecret classifies one stored value against the configured cipher.
// Only a well-formed envelope can report an algorithm mismatch.
func (s *Service) storedSecret(ref Reference, stored string) StoredSecret {
	entry := StoredSecret{Reference: ref, Encrypted: isEnvelope(stored)}
	if entry.Encrypted {
		if env, err := decodeEnvelope(stored); err == nil {
			entry.AlgorithmMismatch = !env.sealedBy(s.cipher.Algorithm())
		}
	}
	return entry
}
