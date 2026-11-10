package cli

import (
	"errors"
	"fmt"

	"github.com/go-envx/envx/app/internal/core/validate"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/str"
	"github.com/spf13/cobra"
)

// errValidationFailed is returned when the graded report fails, so the process
// boundary maps it to a non-zero exit code. The findings themselves are already
// rendered, so this carries only the terse verdict.
var errValidationFailed = errors.New("validation failed")

const (
	validateUsage = "validate"
	validateShort = "Check every project and environment for resolution and store problems"
	validateLong  = `
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
	validateExample = `
		envx validate
		envx validate --strict
		envx validate --output=json

		# a pre-commit hook: run only offline store checks, no environment merge
		envx validate --secret-is-not-encrypted --public-key-is-missing
	`
)

// NewValidateCommand builds the "validate" command.
func NewValidateCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     validateUsage,
		Short:   validateShort,
		Long:    str.Dedent(validateLong),
		Example: str.Dedent(validateExample, 2),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Extract command-line flag values.
			configPath := workspacecli.ConfigPath(cmd)
			strict := strictFlag.Value(cmd)
			output := outputFlag.Value(cmd)
			settings := workspacecli.Settings(cmd)

			// Obtain the validate service using the configuration path.
			validateService, err := f.NewValidateService(configPath)
			if err != nil {
				return err
			}

			// Perform the workspace validation, selecting only the checks whose
			// flags were set.
			report, err := validateService.Validate(validate.ValidateParams{
				Strict:   strict,
				Selected: selectedChecks(cmd),
				Options:  settings,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the result of the workspace validation.
			if err := outputValidate(console, report, output); err != nil {
				return err
			}

			// Signal a non-zero exit when the graded report fails; the findings are
			// already rendered, so only the verdict propagates.
			if report.Failed {
				return errValidationFailed
			}
			return nil
		},
	}

	// Bind the command-line flags. The environment is not among the settings:
	// validate diagnoses every declared environment rather than selecting one.
	outputFlag.Bind(cmd)
	strictFlag.Bind(cmd)
	workspacecli.BindSettings(cmd, "env")
	bindSelectionFlags(cmd)

	return cmd
}

// validateRenderer renders a validate.Report to the console.
type validateRenderer struct {
	console *printer.Printer
}

// outputValidate renders the findings in table or JSON format. An unrecognized
// format is rejected so a typo like --output=jsonn fails loudly.
func outputValidate(
	console *printer.Printer,
	report validate.Report,
	format string,
) error {
	render := validateRenderer{
		console: console,
	}

	switch format {
	case "", "table":
		return render.table(report)
	case "json":
		return render.json(report)
	default:
		return fmt.Errorf("invalid output format %q (want table or json)", format)
	}
}
