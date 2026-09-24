package cli

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/core"
	engine "github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/utils/cliflags"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
	"github.com/spf13/cobra"
)

const (
	usage = "emit <project> --target <format>"
	short = "Render a resolved environment to a delivery target"
	long  = `
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
	example = `
		envx emit api-service --target dotenv
		envx emit api-service --target json --only secrets --env production
		envx emit api-service --target k8s --env production
		envx emit api-service --target k8s-bundle --only config
		envx emit api-service --target k8s-bundle --only secrets
		envx emit api-service --target k8s-bundle --only config --name web
	`
)

// NewEmitCmd builds the "emit" command, which parses the project and flags,
// resolves and reveals the environment, and renders it to the selected target on
// stdout or a chosen file.
func NewEmitCmd() *cobra.Command {
	var (
		target string
		name   string
		output string
		only   string
		key    string
	)

	cmd := &cobra.Command{
		Use:     usage,
		Short:   short,
		Long:    str.Dedent(long),
		Example: str.Dedent(example, 2),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate the target and its --name relationship up front so an unknown
			// format or a misused --name fails before any resolution work.
			parsedTarget, err := engine.ParseTarget(target)
			if err != nil {
				return err
			}
			nameSet := cmd.Flags().Changed(Name.Name)
			if err := validateNameUsage(parsedTarget, nameSet); err != nil {
				return err
			}
			keySet := cmd.Flags().Changed(Key.Name)
			if err := validateKeyUsage(parsedTarget, key, keySet); err != nil {
				return err
			}

			// Resolve which slice(s) to emit up front so an invalid --only value
			// fails before any resolution work.
			includeSecrets, includeConfig, err := resolveSlice(only)
			if err != nil {
				return err
			}

			input := core.GetInput(cmd.Flags())

			// The printer carries the secret-file warning to stderr, styled
			// consistently with the rest of the CLI; the rendered manifest itself
			// still goes to stdout unstyled so it stays pipe-clean.
			pr := printer.New(printer.Options{
				Out: cmd.OutOrStdout(),
				Err: cmd.ErrOrStderr(),
			})

			return execute(actionParams{
				Project:        args[0],
				Target:         parsedTarget,
				Name:           name,
				IncludeSecrets: includeSecrets,
				IncludeConfig:  includeConfig,
				Key:            key,
				OutputPath:     output,
			}, input, cmd.OutOrStdout(), pr)
		},
	}

	env.RegisterFlags(cmd.Flags(),
		env.WithEnv,
		env.WithRequireOverlays,
		env.WithPrefix,
		env.WithSuffix,
		env.WithDelimiter,
		env.WithOverload,
		env.WithReferencePattern,
	)

	cliflags.BindString(cmd.Flags(), &target, &Target)
	cliflags.BindString(cmd.Flags(), &name, &Name)
	cliflags.BindString(cmd.Flags(), &output, &Output)
	cliflags.BindString(cmd.Flags(), &only, &Only)
	cliflags.BindString(cmd.Flags(), &key, &Key)
	_ = cmd.MarkFlagRequired(Target.Name)

	return cmd
}

// NewCommand is an alias for NewEmitCmd.
func NewCommand() *cobra.Command {
	return NewEmitCmd()
}

// Slice values accepted by --only.
const (
	onlySecrets = "secrets"
	onlyConfig  = "config"
)

// resolveSlice translates the --only flag into the two slice selectors: an empty
// value (the default) emits everything, "secrets" or "config" restricts to that
// one slice, and any other value is rejected before resolution begins.
func resolveSlice(only string) (includeSecrets, includeConfig bool, err error) {
	switch only {
	case "":
		return true, true, nil
	case onlySecrets:
		return true, false, nil
	case onlyConfig:
		return false, true, nil
	default:
		return false, false, fmt.Errorf(
			"invalid --only value %q (want %q or %q)", only, onlySecrets, onlyConfig,
		)
	}
}

// validateKeyUsage rejects --key on any target but k8s-bundle, since the data key
// only exists for a bundled resource; the multi-key k8s target names keys after
// the values, and dotenv/json have no keyed resource at all. keySet reports
// whether the flag was explicitly given.
func validateKeyUsage(target engine.Target, key string, keySet bool) error {
	if !keySet {
		return nil
	}
	if target != engine.TargetK8sBundle {
		return fmt.Errorf("--key applies only to the k8s-bundle target, not %q", target)
	}
	// The extension picks the body format, so reject an unknown one up front.
	return engine.ValidateBundleKey(key)
}

// validateNameUsage guards the --name/target relationship: the k8s targets accept
// a name base (defaulting to the project when omitted), and the plain targets
// reject one. nameSet reports whether the flag was explicitly given, so passing
// --name to dotenv/json is an error even though those renderers never read it.
func validateNameUsage(target engine.Target, nameSet bool) error {
	if target.Kubernetes() {
		return nil
	}
	if nameSet {
		return fmt.Errorf("--name applies only to the k8s targets, not %q", target)
	}
	return nil
}
