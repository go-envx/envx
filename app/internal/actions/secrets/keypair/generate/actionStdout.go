package generate

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/go-envx/envx/app/internal/cipher"
	"github.com/go-envx/envx/app/internal/config"
)

// -------------------------------------------------------------------------------------

// actionStdoutResult contains key material explicitly requested for terminal output.
type actionStdoutResult struct {
	// Group is the optional normalized group label shown with the generated keypair.
	Group string
	// Keypair contains the public and private values for this explicit handoff.
	Keypair cipher.Keypair
}

// -------------------------------------------------------------------------------------

// executeStdout generates a keypair through the configured cipher when a
// workspace exists, or through the application default without one. It never
// constructs or mutates a secrets manager.
func executeStdout(p actionParams, in *config.Input) (actionStdoutResult, error) {
	normalizedGroup, err := normalizeGroup(p.Group)
	if err != nil {
		return actionStdoutResult{}, err
	}

	selectedCipher, err := config.NewConfiguredCipher(in)
	if err != nil {
		return actionStdoutResult{}, err
	}
	pair, err := selectedCipher.Keypair()
	if err != nil {
		return actionStdoutResult{}, fmt.Errorf("generating keypair: %w", err)
	}
	return actionStdoutResult{
		Group:   normalizedGroup,
		Keypair: pair,
	}, nil
}

// -------------------------------------------------------------------------------------

// renderStdout prints a keypair without file-oriented status. The group label is
// omitted when the keypair was generated without an assigned group.
func renderStdout(w io.Writer, result actionStdoutResult) error {
	if result.Group != "" {
		if _, err := fmt.Fprintf(w, "group: %s\n", result.Group); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(
		w,
		"public_key: %s\nprivate_key: %s\n",
		result.Keypair.PublicKey,
		result.Keypair.PrivateKey,
	)
	return err
}

// -------------------------------------------------------------------------------------

// renderStdoutNotice explains why an unassigned stdout keypair was not persisted.
func renderStdoutNotice(w io.Writer) error {
	_, err := fmt.Fprintln(
		w,
		"No group provided; this keypair was not stored. Run "+
			"envx secrets keypair generate GROUP to generate and store a "+
			"managed keypair, or copy these values into secrets.yaml and "+
			"envx.keys manually.",
	)
	return err
}

// -------------------------------------------------------------------------------------

// normalizeGroup validates and canonicalizes a supplied group label.
// An empty label is preserved for an unassigned stdout keypair.
func normalizeGroup(group string) (string, error) {
	if group == "" {
		return "", nil
	}
	if strings.TrimSpace(group) == "" {
		return "", errors.New("secret group is empty")
	}
	invalidGroup := strings.ContainsFunc(group, unicode.IsSpace) ||
		strings.ContainsAny(group, "/=")
	if invalidGroup {
		return "", fmt.Errorf("invalid secret group %q", group)
	}
	return strings.ToLower(group), nil
}
