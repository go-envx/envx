// Package bitwarden resolves bws:// references with the Bitwarden Secrets
// Manager CLI.
package bitwarden

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/lib/hostenv"
	"github.com/go-envx/envx/app/internal/lib/process"
)

// program is the Bitwarden Secrets Manager CLI executable.
const program = "bws"

// tokenVar names the variable holding the machine account access token.
const tokenVar = "BWS_ACCESS_TOKEN"

// cliEnv selects the CLI's own variables, such as BWS_ACCESS_TOKEN.
var cliEnv = regexp.MustCompile(`(?i)^BWS_`)

// errorCause matches the first cause line bws prints under its "Error:" line.
var errorCause = regexp.MustCompile(`(?m)^\s*0:\s*(\S.*)$`)

// uuidPattern matches a canonical UUID, the form of project and secret IDs.
var uuidPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// Options configures the provider.
type Options struct {
	// Run executes the CLI and returns its stdout; nil uses process.Output.
	Run external.CommandRunner
	// LookPath locates the CLI offline; nil uses exec.LookPath.
	LookPath func(file string) (string, error)
}

// Provider fetches bws:// references by secret ID or by project and key. It
// caches listings for its lifetime, which is one operation.
type Provider struct {
	// run executes the CLI and returns its stdout.
	run external.CommandRunner
	// lookPath locates the CLI offline.
	lookPath func(file string) (string, error)
	// env is the minimal child environment.
	env map[string]string
	// hasToken reports whether env holds a non-empty access token.
	hasToken bool
	// projects caches the accessible projects once listed.
	projects []project
	// secrets caches each listed project's secrets by project ID.
	secrets map[string][]secret
}

// project is one entry of `bws project list`.
type project struct {
	// ID identifies the project.
	ID string `json:"id"`
	// Name is the project's display name, not necessarily unique.
	Name string `json:"name"`
}

// secret is one entry of `bws secret get` or `bws secret list`.
type secret struct {
	// ID identifies the secret.
	ID string `json:"id"`
	// Key is the secret's name, not necessarily unique within a project.
	Key string `json:"key"`
	// Value is the plaintext.
	Value string `json:"value"`
}

// New constructs a provider whose CLI runs with the host's base and BWS_*
// variables only.
func New(host hostenv.Snapshot, opts Options) *Provider {
	run := opts.Run
	if run == nil {
		run = process.Output
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	env := process.MinimalEnv(host.All(), cliEnv)
	hasToken := false
	for name, val := range env {
		if strings.EqualFold(name, tokenVar) && val != "" {
			hasToken = true
		}
	}
	return &Provider{
		run:      run,
		lookPath: lookPath,
		env:      env,
		hasToken: hasToken,
		secrets:  make(map[string][]secret),
	}
}

// Scheme returns "bws".
func (p *Provider) Scheme() string {
	return external.SchemeBitwarden
}

// Qualify accepts a secret ID or "<project>/<key>" without a "#field"
// fragment; references need no qualification.
func (p *Provider) Qualify(
	ref external.Reference, _ string,
) (external.Reference, error) {
	if ref.Field != "" {
		return external.Reference{}, fmt.Errorf(
			"%w: bws references do not support #<field>",
			external.ErrInvalidReference,
		)
	}
	if !strings.Contains(ref.Path, "/") && !uuidPattern.MatchString(ref.Path) {
		return external.Reference{}, fmt.Errorf(
			"%w: want bws://<secret-id> or bws://<project>/<key>",
			external.ErrInvalidReference,
		)
	}
	return ref, nil
}

// CheckAvailable reports a missing access token or CLI.
func (p *Provider) CheckAvailable() error {
	if !p.hasToken {
		return tokenError()
	}
	if _, err := p.lookPath(program); err != nil {
		return notFoundError()
	}
	return nil
}

// Fetch reads one reference: a secret ID, or "<project>/<key>" where the
// project is a name or ID and the key may contain slashes.
func (p *Provider) Fetch(ctx context.Context, ref external.Reference) (string, error) {
	if _, err := p.Qualify(ref, ""); err != nil {
		return "", err
	}
	if !p.hasToken {
		return "", tokenError()
	}
	projectRef, key, byKey := strings.Cut(ref.Path, "/")
	if !byKey {
		return p.secretByID(ctx, ref.Path)
	}
	projectID, err := p.projectID(ctx, projectRef)
	if err != nil {
		return "", err
	}
	return p.secretByKey(ctx, projectID, projectRef, key)
}

// secretByID fetches one secret directly.
func (p *Provider) secretByID(ctx context.Context, id string) (string, error) {
	var found secret
	if err := p.query(ctx, &found, "secret", "get", id); err != nil {
		return "", err
	}
	return found.Value, nil
}

// secretByKey selects the single secret with key in the project, listing the
// project's secrets once.
func (p *Provider) secretByKey(
	ctx context.Context, projectID, projectRef, key string,
) (string, error) {
	listed, ok := p.secrets[projectID]
	if !ok {
		if err := p.query(ctx, &listed, "secret", "list", projectID); err != nil {
			return "", err
		}
		p.secrets[projectID] = listed
	}
	var matches []secret
	for _, s := range listed {
		if s.Key == key {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"%w: no secret with key %q in project %q",
			external.ErrReferenceUnresolved, key, projectRef,
		)
	case 1:
		return matches[0].Value, nil
	default:
		ids := make([]string, len(matches))
		for i, s := range matches {
			ids[i] = s.ID
		}
		return "", fmt.Errorf(
			"%w: %d secrets with key %q in project %q; "+
				"reference one by ID, such as bws://%s (IDs: %s)",
			external.ErrReferenceUnresolved, len(matches), key, projectRef,
			ids[0], strings.Join(ids, ", "),
		)
	}
}

// projectID returns ref when it is an ID, or else the ID of the single
// accessible project named ref, listing projects once.
func (p *Provider) projectID(ctx context.Context, ref string) (string, error) {
	if uuidPattern.MatchString(ref) {
		return ref, nil
	}
	if p.projects == nil {
		var listed []project
		if err := p.query(ctx, &listed, "project", "list"); err != nil {
			return "", err
		}
		p.projects = listed
	}
	var ids []string
	for _, proj := range p.projects {
		if proj.Name == ref {
			ids = append(ids, proj.ID)
		}
	}
	switch len(ids) {
	case 0:
		return "", fmt.Errorf(
			"%w: no accessible project named %q",
			external.ErrReferenceUnresolved, ref,
		)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf(
			"%w: %d projects named %q; reference one by ID (IDs: %s)",
			external.ErrReferenceUnresolved, len(ids), ref, strings.Join(ids, ", "),
		)
	}
}

// query runs one bws subcommand with JSON output and decodes stdout into dst.
// Errors never quote stdout, which may hold secret values.
func (p *Provider) query(ctx context.Context, dst any, args ...string) error {
	command := program + " " + strings.Join(args[:2], " ")
	out, err := p.run(ctx, process.OutputCommand{
		Args: append(
			append([]string{program}, args...), "--output", "json", "--color", "no",
		),
		Env: p.env,
	})
	if err != nil {
		return commandError(command, err)
	}
	if err := json.Unmarshal(out, dst); err != nil {
		return fmt.Errorf(
			"%w: %s: unexpected output format", external.ErrReferenceUnresolved, command,
		)
	}
	return nil
}

// commandError maps a CLI failure onto the external sentinels.
func commandError(command string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return notFoundError()
	}
	if exitErr, ok := errors.AsType[*process.ExitError](err); ok {
		return fmt.Errorf(
			"%w: %s: exit status %d: %s",
			external.ErrReferenceUnresolved, command, exitErr.Code,
			summarizeStderr(exitErr.Stderr),
		)
	}
	return err
}

// summarizeStderr prefers the first error cause over the bare "Error:" line.
func summarizeStderr(stderr string) string {
	if match := errorCause.FindStringSubmatch(stderr); match != nil {
		return external.SummarizeStderr(match[1])
	}
	return external.SummarizeStderr(stderr)
}

// notFoundError reports the CLI missing from PATH.
func notFoundError() error {
	return fmt.Errorf(
		"%w: %s CLI not found on PATH; install Bitwarden Secrets Manager CLI",
		external.ErrProviderUnavailable, program,
	)
}

// tokenError reports a missing access token.
func tokenError() error {
	return fmt.Errorf("%w: %s is not set", external.ErrProviderUnavailable, tokenVar)
}
