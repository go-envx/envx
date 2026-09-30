package filestore

// secretsYAML represents the root schema of a secrets document.
type secretsYAML struct {
	PublicKeys map[string]string            `yaml:"public-keys"`
	Secrets    map[string]map[string]string `yaml:"secrets"`
}

// secret identifies one stored value by its group and key.
type secret struct {
	// Group is the stored key-group name.
	Group string
	// Key is the entry name within Group.
	Key string
	// Value is the stored plaintext or ciphertext value.
	Value string
}
