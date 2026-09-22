package validate

import (
	"errors"

	"github.com/go-envx/envx/app/internal/flags"
	"github.com/go-envx/envx/app/internal/schema"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	engine "github.com/go-envx/envx/app/internal/validate"
	"github.com/spf13/cobra"
)

// errValidationFailed is returned when the graded report fails, so the process
// boundary maps it to a non-zero exit code. The findings themselves are already
// rendered, so this carries only the terse verdict.
var errValidationFailed = errors.New("validation failed")

const (
	usage = "validate"
	short = "Check every project and environment for resolution and store problems"
	long  = `
		Validate resolves every project against every declared environment and
		reports the problems a workspace-wide gate should catch before deploy. Its
		checks fall into two groups by the files they read, which is also their cost:

		  store      offline, instant; reads only secrets.yaml and the keys file
		             (unencrypted value, algorithm mismatch, unused value, missing
		             public key, invalid private key, unavailable private key)
		  resolution merges each project and environment (dangling or undecryptable
		             reference, invalid reference, variable-substitution cycle,
		             undefined variable, and a key an environment overlay declares
		             but its namespace base file does not)

		By default every check runs. Passing any per-check flag runs only the named
		checks and skips the cost of the rest, so a pre-commit hook can select just
		the offline store checks and pay for no environment merge. Each per-check
		flag is named identically to its severity key in the envx.yaml validate
		block, so --secret-is-not-encrypted selects the check configured by
		validate.secret_is_not_encrypted; severity itself is set only in envx.yaml,
		never on the command line.

		It never materializes plaintext (every reference is diagnosed through the
		masked dry-run path) and it never aborts on a single failure: every problem
		is collected and reported together. Errors always fail the command; --strict
		additionally fails it on warnings (chiefly a private key that is absent in
		this context, which is normal on a developer laptop but a failure for a
		full-CI gate). Use --output=json for machine-readable output.
	`
	example = `
		envx validate
		envx validate --strict
		envx validate --output=json

		# a pre-commit hook: run only offline store checks, no environment merge
		envx validate --secret-is-not-encrypted --public-key-is-missing
	`
)

// NewCommand builds the "validate" command. It resolves the whole workspace,
// runs the workspace-wide diagnosis, renders the findings, and returns a failure
// error when the graded report fails so the process exits non-zero.
func NewCommand() *cobra.Command {
	var output string
	var strict bool

	// Register one boolean selection flag per check, named identically to the
	// check's envx.yaml severity key (kebab-case), so --secret-is-not-encrypted
	// selects the check configured by validate.secret_is_not_encrypted. The value
	// pointers are read back into the selected-check set inside RunE.
	selections := make(map[string]*bool)

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// get the flag inputs
			input := flags.GetInput(cmd.Flags())

			// execute the action, selecting only the checks whose flags were set
			report, err := execute(actionParams{
				Strict:   strict,
				Selected: selectedChecks(selections),
			}, input)
			if err != nil {
				return err
			}

			// render the findings through the shared printer
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})
			if err := render(&renderParams{
				Printer: pr,
				Report:  report,
				Format:  output,
			}); err != nil {
				return err
			}

			// signal a non-zero exit when the graded report fails; the findings are
			// already rendered, so only the verdict propagates.
			if report.Failed {
				return errValidationFailed
			}
			return nil
		},
	}

	flags.Register(cmd.Flags(),
		flags.WithRequireOverlays,
		flags.WithPrefix,
		flags.WithSuffix,
		flags.WithDelimiter,
		flags.WithOverload,
		flags.WithReferencePattern,
	)

	flags.BindString(cmd.Flags(), &output, &schema.Output)
	flags.BindBool(cmd.Flags(), &strict, &schema.Strict)

	registerSelectionFlags(cmd, selections)

	return cmd
}

// registerSelectionFlags adds one boolean selection flag per check, binding each
// into selections keyed by the check's canonical code so RunE can read which were
// set. The flag name is the check code's kebab-case form, matching its envx.yaml
// severity key so the flag and the config property can never drift apart.
func registerSelectionFlags(cmd *cobra.Command, selections map[string]*bool) {
	for _, check := range engine.Checks() {
		usage := "run only this check: " + check.Summary + " [" + string(check.Group) + "]"
		selections[check.Code] = cmd.Flags().Bool(engine.FlagName(check.Code), false, usage)
	}
}

// selectedChecks collapses the bound selection-flag pointers into a set of the
// codes whose flags were set. An empty result means no selection was made, which
// the engine reads as "run every check".
func selectedChecks(selections map[string]*bool) map[string]bool {
	selected := make(map[string]bool)
	for code, value := range selections {
		if *value {
			selected[code] = true
		}
	}
	return selected
}
