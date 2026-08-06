package privatekey

// -------------------------------------------------------------------------------------

// Destination receives newly generated private-key material.
type Destination interface {
	// Write hands off privateKey for group.
	Write(group, privateKey string) error
}

// -------------------------------------------------------------------------------------

// FilePath reports the path when destination exposes a private key file location.
func FilePath(destination Destination) (string, bool) {
	pather, ok := destination.(interface{ Path() string })
	if !ok {
		return "", false
	}
	return pather.Path(), true
}
