package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-envx/envx/app/internal/core/emit"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/atomicfile"
	"github.com/go-envx/envx/app/internal/lib/filex"
	"github.com/go-envx/envx/app/internal/lib/printer"
	"github.com/go-envx/envx/app/internal/lib/str"
	"github.com/spf13/cobra"
)

const (
	emitUsage = "emit <project> --target <format>"
	emitShort = "Render a resolved environment to a delivery target"
	emitLong  = `
		Emit fully resolves and decrypts a single project environment and renders
		it to a delivery target. It reveals every value through the ordinary
		resolution path, so a value that cannot be resolved or decrypted aborts the
		command before any output is written — emit never produces a partial result.

		The --target flag selects the output shape:
		  k8s         canonical Kubernetes resources — a ConfigMap and/or Secret with
		              each value its own data key (consume with envFrom)
		  k8s-bundle  the slice packed into a single data key holding one JSON blob,
		              so the resource can be volume-mounted as one file
		  json        a flat JSON object
		  dotenv      KEY=value lines

		The --only flag restricts the output to one slice — "secrets" for the
		secret-derived values or "config" for the plain ones; omitting it emits
		everything (for k8s, both a Secret and a ConfigMap).

		By default the k8s targets name their resources <project>-config /
		<project>-secrets (a merged bundle is <project>-config-secrets); an explicit
		--name is used verbatim instead. A k8s-bundle's data key — also the mounted
		filename — defaults to config.json (secrets.json for a secrets-only bundle);
		--key overrides it, and its extension (.json or .env) picks the body format.

		The target environment is chosen by --env, the ENVX_ENV env var, a manifest
		env setting, or the first environment declared in envx.yaml. Output goes to
		stdout unless --output names a file, which is written with private
		permissions because it may carry plaintext secrets.
	`
	emitExample = `
		envx emit api-service --target dotenv
		envx emit api-service --target json --only secrets --env production
		envx emit api-service --target k8s --env production
		envx emit api-service --target k8s-bundle --only config
		envx emit api-service --target k8s-bundle --only secrets
		envx emit api-service --target k8s-bundle --only config --name web
	`
)

// NewEmitCommand builds the "emit" command.
func NewEmitCommand(f Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     emitUsage,
		Short:   emitShort,
		Long:    str.Dedent(emitLong),
		Example: str.Dedent(emitExample, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract command-line arguments.
			project := args[0]

			// Extract command-line flag values.
			configPath := workspacecli.ConfigPath(cmd)
			options := emit.EmitOptions{
				Target: emit.Target(targetFlag.Value(cmd)),
				Only:   onlyFlag.Value(cmd),
				Name:   nameFlag.Value(cmd),
				Key:    keyFlag.Value(cmd),
			}
			outputPath := outputFlag.Value(cmd)
			settings := workspacecli.Settings(cmd)

			// Fail fast, before any decryption, if the output directory is missing —
			// emit writes the named file but does not create directories, so a bad
			// path is surfaced clearly rather than as a temp-file error after the
			// work is done.
			if outputPath != "" {
				dir := filepath.Dir(outputPath)
				if err := filex.AssertDirectoryExists(dir); err != nil {
					return fmt.Errorf("output directory %s does not exist", dir)
				}
			}

			// Obtain the emit service using the configuration path.
			emitService, err := f.NewEmitService(configPath)
			if err != nil {
				return err
			}

			// Render the environment. Emit validates the options first and fails
			// closed, so a value that cannot be resolved aborts before any output.
			result, err := emitService.Emit(emit.EmitParams{
				Project:  project,
				Options:  options,
				Settings: settings,
			})
			if err != nil {
				return err
			}

			// Initialize the console printer for output. It carries the confirmation
			// and the secret-file warning, while the rendered manifest itself goes to
			// stdout unstyled so it stays pipe-clean.
			console := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			// Output the rendered result to stdout or the chosen file.
			return outputEmit(
				console, cmd.OutOrStdout(), result, outputPath, project, options,
			)
		},
	}

	// Bind the command-line flags.
	targetFlag.Bind(cmd)
	onlyFlag.Bind(cmd)
	nameFlag.Bind(cmd)
	keyFlag.Bind(cmd)
	outputFlag.Bind(cmd)
	workspacecli.BindSettings(cmd)
	_ = cmd.MarkFlagRequired(targetFlag.Name)

	return cmd
}

// outputEmit delivers the rendered bytes: to stdout when outputPath is empty, or
// atomically to the file otherwise. Emitting to stdout stays quiet so the
// manifest can be piped cleanly; the confirmation is reserved for a file target,
// and when that file carries secret material a warning follows it, since the
// written file is unencrypted and must not be committed.
func outputEmit(
	console *printer.Printer,
	stdout io.Writer,
	result emit.EmitResult,
	outputPath string,
	project string,
	options emit.EmitOptions,
) error {
	if outputPath == "" {
		_, err := stdout.Write(result.Data)
		return err
	}

	// The render may carry plaintext secrets, so persist it with private
	// permissions rather than the world-readable default.
	if err := atomicfile.WritePrivate(outputPath, result.Data); err != nil {
		return err
	}

	// Confirm where the file landed; a k8s-bundle also points to the volume docs.
	slice := "config and secrets"
	switch options.Only {
	case emit.OnlySecrets:
		slice = "secrets"
	case emit.OnlyConfig:
		slice = "config"
	}
	summary := fmt.Sprintf("Wrote %s %s to %s.", project, slice, outputPath)
	if options.Target == emit.TargetKubernetesBundle {
		summary += " Read the docs to learn how to use the file in a Kubernetes volume."
	}
	if err := console.LogMessage(summary); err != nil {
		return err
	}

	// When the file carries secret material — the values are unencrypted
	// (plaintext, or base64 in a Secret) — add a blank line and a stderr caution
	// not to commit it.
	if !result.ContainsSecrets {
		return nil
	}
	if err := console.LogBlank(); err != nil {
		return err
	}
	return console.LogWarning(
		"the output carries unencrypted secret values; do not commit it",
	)
}
