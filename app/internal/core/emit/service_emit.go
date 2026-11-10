package emit

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/workspace"
)

// Target identifies an emit output format.
type Target string

// Supported emit targets.
const (
	// TargetJSON renders a flat JSON object.
	TargetJSON Target = "json"
	// TargetDotenv renders KEY=value lines.
	TargetDotenv Target = "dotenv"
	// TargetKubernetes renders a ConfigMap and/or Secret with one key per value.
	TargetKubernetes Target = "k8s"
	// TargetKubernetesBundle renders one resource holding a single data key.
	TargetKubernetesBundle Target = "k8s-bundle"
)

// Slice selections accepted by EmitOptions.Only.
const (
	// OnlyAll selects every value; empty is equivalent.
	OnlyAll = "all"
	// OnlyConfig selects values that do not derive from secrets.
	OnlyConfig = "config"
	// OnlySecrets selects values that derive from secrets.
	OnlySecrets = "secrets"
)

// EmitOptions configures emit rendering.
type EmitOptions struct {
	// Target selects the output format.
	Target Target
	// Only selects a slice: empty or OnlyAll, OnlyConfig, or OnlySecrets.
	Only string
	// Name is an explicit Kubernetes resource name used verbatim; empty derives
	// suffixed names from the project.
	Name string
	// Key is the Kubernetes bundle data key; empty uses the slice default.
	Key string
}

// EmitParams selects the environment and rendering options.
type EmitParams struct {
	// Project names the declared project; empty selects the only project.
	Project string
	// Options configures rendering.
	Options EmitOptions
	// Settings are setting overrides that win over the manifest.
	Settings workspace.Settings
}

// EmitResult is a rendered emit output. Data holds materialized values and
// must not be logged.
type EmitResult struct {
	// Data is the complete rendered output.
	Data []byte
	// ContainsSecrets reports whether Data carries secret-derived values.
	ContainsSecrets bool
}

// Emit validates the options, reveals every winner, and renders the complete
// output to memory. Any value that does not resolve fails the operation before
// rendering, so no partial output is produced. Host-only keys are excluded.
func (s *Service) Emit(params EmitParams) (EmitResult, error) {
	if err := s.deps.Renderer.Validate(params.Options); err != nil {
		return EmitResult{}, err
	}
	explained, err := s.deps.Env.Explain(env.ExplainParams{
		Project: params.Project,
		Reveal:  true,
		Options: params.Settings,
	})
	if err != nil {
		return EmitResult{}, err
	}

	entries := explained.Entries
	var unresolved []string
	containsSecrets := false
	for i := range entries {
		if !entries[i].Evaluation.Resolved {
			unresolved = append(unresolved, entries[i].Key)
		}
		containsSecrets = containsSecrets || entries[i].Secret
	}
	if len(unresolved) > 0 {
		return EmitResult{}, fmt.Errorf(
			"cannot emit: %w: %d value(s) did not resolve: %s",
			env.ErrUnresolved, len(unresolved), strings.Join(unresolved, ", "),
		)
	}

	var buffer bytes.Buffer
	if err := s.deps.Renderer.Render(
		&buffer, explained.Project, entries, params.Options,
	); err != nil {
		return EmitResult{}, err
	}
	return EmitResult{
		Data:            buffer.Bytes(),
		ContainsSecrets: containsSecrets && params.Options.Only != OnlyConfig,
	}, nil
}
