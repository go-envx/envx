package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/features/emit"
	"github.com/go-envx/envx/app/internal/features/env"
	"github.com/go-envx/envx/app/internal/shared/flags"
	"github.com/go-envx/envx/app/internal/utils/filex"
	"github.com/go-envx/envx/app/internal/utils/printer"
	"github.com/go-envx/envx/app/internal/utils/str"
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

// Slice values accepted by --only.
const (
	onlySecrets = "secrets"
	onlyConfig  = "config"
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
			fs := cmd.Flags()
			configPath := flags.Config.Get(fs)
			target := targetFlag.Get(fs)
			only := onlyFlag.Get(fs)
			name := nameFlag.Get(fs)
			key := keyFlag.Get(fs)
			output := outputFlag.Get(fs)
			envOptions := getEnvOptions(fs)

			// Validate the target and its --name relationship up front so an unknown
			// format or a misused --name fails before any resolution work.
			parsedTarget, err := emit.ParseTarget(target)
			if err != nil {
				return err
			}
			if err := validateNameUsage(parsedTarget, fs.Changed(nameFlag.Name)); err != nil {
				return err
			}
			if err := validateKeyUsage(parsedTarget, key, fs.Changed(keyFlag.Name)); err != nil {
				return err
			}

			// Resolve which slice(s) to emit up front so an invalid --only value
			// fails before any resolution work.
			includeSecrets, includeConfig, err := resolveSlice(only)
			if err != nil {
				return err
			}

			// A k8s resource name defaults to the project, with the slice suffix
			// appended so one project yields app-config / app-secrets without a flag.
			// An explicit --name is honored verbatim instead.
			nameBase := name
			exactName := name != ""
			if nameBase == "" {
				nameBase = project
			}

			// A k8s-bundle data key (and mounted filename) defaults per slice, so the
			// common case needs no --key: config.json, or secrets.json for a
			// secrets-only bundle.
			if parsedTarget == emit.TargetK8sBundle && key == "" {
				key = defaultBundleKey(includeSecrets, includeConfig)
			}

			// Fail fast, before any decryption, if the output directory is missing —
			// emit writes the named file but does not create directories, so a bad
			// path is surfaced clearly rather than as a temp-file error after the
			// work is done.
			if err := checkOutputDir(output); err != nil {
				return err
			}

			// Obtain the environment service using the configuration path.
			envService, err := f.EnvService(configPath)
			if err != nil {
				return err
			}

			// Reveal and classify every winning value. Explain never aborts on a
			// per-key failure — it carries the status instead — so emit enforces its
			// own fail-closed contract in entriesFromExplanation below.
			result, err := envService.Explain(env.ExplainParams{
				Project: project,
				Reveal:  true,
				Options: envOptions,
			})
			if err != nil {
				return err
			}

			entries, err := entriesFromExplanation(result)
			if err != nil {
				return err
			}

			// Buffer the render so no partial output escapes on a formatting error and
			// a file target is committed atomically.
			var buffer bytes.Buffer

			// Obtain the emit service.
			emitService, err := f.EmitService()
			if err != nil {
				return err
			}

			renderParams := emit.RenderParams{
				Entries:        entries,
				Target:         parsedTarget,
				NameBase:       nameBase,
				ExactName:      exactName,
				IncludeSecrets: includeSecrets,
				IncludeConfig:  includeConfig,
				Key:            key,
				Writer:         &buffer,
			}
			if err := emitService.Render(renderParams); err != nil {
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
				console, cmd.OutOrStdout(), buffer.Bytes(), output, project, &renderParams,
			)
		},
	}

	// Bind the command-line flags.
	{
		fs := cmd.Flags()
		flags.Bind(fs, &targetFlag)
		flags.Bind(fs, &onlyFlag)
		flags.Bind(fs, &nameFlag)
		flags.Bind(fs, &keyFlag)
		flags.Bind(fs, &outputFlag)
		bindEnvOptionsFlags(fs)
		_ = cmd.MarkFlagRequired(targetFlag.Name)
	}

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
	rendered []byte,
	outputPath string,
	project string,
	params *emit.RenderParams,
) error {
	if outputPath == "" {
		_, err := stdout.Write(rendered)
		return err
	}

	// The render may carry plaintext secrets, so persist it with private
	// permissions rather than the world-readable default.
	if err := filex.WriteAtomicPrivate(outputPath, rendered); err != nil {
		return err
	}

	// Confirm where the file landed as normal output — the summary, then (for a
	// k8s-bundle) the docs pointer in the same line.
	summary := wroteSummary(project, outputPath, params)
	if params.Target == emit.TargetK8sBundle {
		summary += " Read the docs to learn how to use the file in a Kubernetes volume."
	}
	if err := console.LogMessage(summary); err != nil {
		return err
	}

	// When the file carries secret material — the values are unencrypted
	// (plaintext, or base64 in a Secret) — add a blank line and a stderr caution
	// not to commit it.
	if !outputCarriesSecrets(params) {
		return nil
	}
	if err := console.LogBlank(); err != nil {
		return err
	}
	return console.LogWarning(
		"the output carries unencrypted secret values; do not commit it",
	)
}

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
func validateKeyUsage(target emit.Target, key string, keySet bool) error {
	if !keySet {
		return nil
	}
	if target != emit.TargetK8sBundle {
		return fmt.Errorf("--key applies only to the k8s-bundle target, not %q", target)
	}
	// The extension picks the body format, so reject an unknown one up front.
	return emit.ValidateBundleKey(key)
}

// validateNameUsage guards the --name/target relationship: the k8s targets accept
// a name base (defaulting to the project when omitted), and the plain targets
// reject one. nameSet reports whether the flag was explicitly given, so passing
// --name to dotenv/json is an error even though those renderers never read it.
func validateNameUsage(target emit.Target, nameSet bool) error {
	if target.Kubernetes() {
		return nil
	}
	if nameSet {
		return fmt.Errorf("--name applies only to the k8s targets, not %q", target)
	}
	return nil
}

// defaultBundleKey picks the default k8s-bundle data key from the selected
// slice: a secrets-only bundle defaults to secrets.json, and config-only or a
// merged bundle to config.json. The .json extension makes the body JSON.
func defaultBundleKey(includeSecrets, includeConfig bool) string {
	if includeSecrets && !includeConfig {
		return "secrets.json"
	}
	return "config.json"
}

// checkOutputDir reports an error when the file's parent directory does not
// exist, so emit refuses to write into a missing path rather than creating
// directories the user did not ask for. An empty path (stdout) always passes.
func checkOutputDir(outputPath string) error {
	if outputPath == "" {
		return nil
	}
	dir := filepath.Dir(outputPath)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("output directory %s does not exist", dir)
	}
	return nil
}

// wroteSummary is the one-line confirmation printed after a file write: which
// project's slice was written, and to which file.
func wroteSummary(project, outputPath string, params *emit.RenderParams) string {
	slice := "config and secrets"
	switch {
	case params.IncludeSecrets && !params.IncludeConfig:
		slice = "secrets"
	case !params.IncludeSecrets && params.IncludeConfig:
		slice = "config"
	}
	return fmt.Sprintf("Wrote %s %s to %s.", project, slice, outputPath)
}

// outputCarriesSecrets reports whether the rendered file will contain
// secret-derived values, so the caller can warn that it must not be committed.
// The file carries secrets only when the secret slice was selected and the
// environment actually holds a secret-derived value; a config-only render never
// triggers the warning.
func outputCarriesSecrets(params *emit.RenderParams) bool {
	if !params.IncludeSecrets {
		return false
	}
	for i := range params.Entries {
		if params.Entries[i].Secret {
			return true
		}
	}
	return false
}

// entriesFromExplanation converts a revealed explanation into emit entries,
// enforcing emit's fail-closed contract: if any value did not resolve to
// plaintext it returns an error naming every unresolved key and no entries, so
// the caller emits nothing. A value classified as a secret reference becomes a
// secret-derived entry, which the Kubernetes split routes into a Secret.
func entriesFromExplanation(result *env.ExplainResult) ([]emit.Entry, error) {
	entries := make([]emit.Entry, 0, len(result.Entries))
	var unresolved []string
	for i := range result.Entries {
		e := &result.Entries[i]
		if !e.Resolution.HasResolved {
			unresolved = append(unresolved, e.Key)
			continue
		}
		entries = append(entries, emit.Entry{
			Key:    e.Key,
			Value:  e.Resolution.Resolved,
			Secret: e.Resolution.Kind == env.KindSecretReference,
		})
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return nil, fmt.Errorf(
			"cannot emit: %d value(s) did not resolve: %s",
			len(unresolved), strings.Join(unresolved, ", "),
		)
	}
	return entries, nil
}
