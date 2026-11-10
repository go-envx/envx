// Package cli implements "envx validate", the workspace-wide gate that resolves
// every project against every declared environment and checks the secrets store.
// It selects which checks run, delegates the diagnosis to
// internal/core/validate, renders the findings, and returns a non-zero exit
// when the graded report fails.
package cli

import (
	"strings"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/validate"
	"github.com/go-envx/envx/app/internal/lib/cliflags"
	"github.com/spf13/cobra"
)

// Flags local to the validate command.
var (
	// strictFlag fails validation on warnings as well as errors.
	strictFlag = cliflags.Spec[bool]{
		Name:  "strict",
		Usage: "fail on warnings as well as errors",
	}

	// outputFlag selects the rendering format.
	outputFlag = cliflags.Spec[string]{
		Name:  "output",
		Short: "o",
		Usage: "output format: table|json",
	}
)

// bindSelectionFlags registers one boolean selection flag per check.
func bindSelectionFlags(cmd *cobra.Command) {
	for _, check := range validate.Checks() {
		selectionFlag(check).Bind(cmd)
	}
}

// selectedChecks returns the codes whose selection flags were set, in catalog
// order. An empty result means no selection was made, which the service reads
// as "run every check".
func selectedChecks(cmd *cobra.Command) []diagnostic.Code {
	var selected []diagnostic.Code
	for _, check := range validate.Checks() {
		if selectionFlag(check).Value(cmd) {
			selected = append(selected, check.Code)
		}
	}
	return selected
}

// selectionFlag describes the flag that selects check. Its name is the check
// code's kebab-case form, matching its envx.yaml severity key so the flag and
// the config property can never drift apart.
func selectionFlag(check validate.Check) cliflags.Spec[bool] {
	return cliflags.Spec[bool]{
		Name:  flagName(check.Code),
		Usage: "run only this check: " + check.Summary + " [" + string(check.Group) + "]",
	}
}

// flagName returns a check's selection flag name: the lowercased code with
// underscores turned into dashes, e.g. SECRET_IS_NOT_ENCRYPTED ->
// "secret-is-not-encrypted". It is the kebab-case twin of the code's config
// severity key, so `validate --secret-is-not-encrypted` selects exactly the check
// whose severity is set by `validate.secret_is_not_encrypted` in envx.yaml.
func flagName(code diagnostic.Code) string {
	return strings.ReplaceAll(strings.ToLower(string(code)), "_", "-")
}

// Factory defines the capabilities required for the validate command.
type Factory interface {
	NewValidateService(configPath string) (*validate.Service, error)
}
