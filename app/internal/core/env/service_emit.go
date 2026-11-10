package env

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

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

// Artifact is a rendered emit output. Data holds materialized values and must
// not be logged.
type Artifact struct {
	// Data is the complete rendered output.
	Data []byte
	// ContainsSecrets reports whether Data carries secret-derived values.
	ContainsSecrets bool
}

// Emit validates the options, reveals every winner, and renders the complete
// output to memory. Any value that does not resolve fails the operation before
// rendering, so no partial artifact is produced. Host-only keys are excluded.
func (s *Service) Emit(params EmitParams) (Artifact, error) {
	if s.deps.FormatRenderer == nil {
		return Artifact{}, errors.New("renderer is required")
	}
	if err := s.deps.FormatRenderer.Validate(params.Options); err != nil {
		return Artifact{}, err
	}
	op, err := s.begin(params.Project, params.Settings)
	if err != nil {
		return Artifact{}, err
	}
	merged, err := s.load(op, false)
	if err != nil {
		return Artifact{}, err
	}
	resolver, err := s.newValueResolver(true, op.host)
	if err != nil {
		return Artifact{}, err
	}

	keys := slices.Sorted(maps.Keys(merged.values))
	entries := newEvaluator(merged, resolver, op, true).entries(keys)
	var unresolved []string
	containsSecrets := false
	for i := range entries {
		if !entries[i].Evaluation.Resolved {
			unresolved = append(unresolved, entries[i].Key)
		}
		containsSecrets = containsSecrets || entries[i].Secret
	}
	if len(unresolved) > 0 {
		return Artifact{}, fmt.Errorf(
			"cannot emit: %w: %d value(s) did not resolve: %s",
			ErrUnresolved, len(unresolved), strings.Join(unresolved, ", "),
		)
	}

	var buffer bytes.Buffer
	if err := s.deps.FormatRenderer.Render(
		&buffer, op.project, entries, params.Options,
	); err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Data:            buffer.Bytes(),
		ContainsSecrets: containsSecrets && params.Options.Only != OnlyConfig,
	}, nil
}
