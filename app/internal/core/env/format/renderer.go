package format

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/env"
)

var _ env.Renderer = Renderer{}

// targets lists every supported target in a stable order.
var targets = []env.Target{
	env.TargetKubernetes,
	env.TargetKubernetesBundle,
	env.TargetJSON,
	env.TargetDotenv,
}

// Renderer implements env.Renderer. The zero value is ready to use.
type Renderer struct{}

// item is one resolved value to render.
type item struct {
	// key is the variable name.
	key string
	// value is the resolved plaintext.
	value string
	// secret marks a secret-derived value.
	secret bool
}

// naming derives Kubernetes resource names.
type naming struct {
	// base is the explicit name or the project name.
	base string
	// exact uses base verbatim instead of appending a slice suffix.
	exact bool
}

// selection is the set of value slices an Only option selects.
type selection struct {
	// secrets selects secret-derived values.
	secrets bool
	// config selects values that do not derive from secrets.
	config bool
}

// Validate rejects unknown targets, unknown slices, and options that do not
// apply to the target.
func (Renderer) Validate(options env.EmitOptions) error {
	if !slices.Contains(targets, options.Target) {
		names := make([]string, len(targets))
		for i, t := range targets {
			names[i] = string(t)
		}
		return fmt.Errorf(
			"%w %q (want one of: %s)",
			ErrUnknownTarget, options.Target, strings.Join(names, ", "),
		)
	}
	if _, err := parseSelection(options.Only); err != nil {
		return err
	}
	if options.Name != "" && !kubernetes(options.Target) {
		return fmt.Errorf(
			"%w: name applies only to the k8s targets, not %q",
			ErrInvalidOption, options.Target,
		)
	}
	if options.Key != "" {
		if options.Target != env.TargetKubernetesBundle {
			return fmt.Errorf(
				"%w: key applies only to the k8s-bundle target, not %q",
				ErrInvalidOption, options.Target,
			)
		}
		if _, err := parseBundleFormat(options.Key); err != nil {
			return err
		}
	}
	return nil
}

// Render writes the selected slice of entries in sorted key order. Kubernetes
// names derive from project with a slice suffix unless options.Name is set.
func (r Renderer) Render(
	out io.Writer, project string, entries []env.ExplainEntry, options env.EmitOptions,
) error {
	if err := r.Validate(options); err != nil {
		return err
	}
	selected, _ := parseSelection(options.Only)
	items := make([]item, len(entries))
	for i := range entries {
		e := &entries[i]
		items[i] = item{key: e.Key, value: e.Evaluation.Value, secret: e.Secret}
	}
	slices.SortFunc(items, func(a, b item) int { return cmp.Compare(a.key, b.key) })

	names := naming{base: cmp.Or(options.Name, project), exact: options.Name != ""}
	if kubernetes(options.Target) && names.base == "" {
		return fmt.Errorf("%w: target %q", ErrMissingNameBase, options.Target)
	}

	switch options.Target {
	case env.TargetJSON:
		return renderJSON(out, selected.filter(items))
	case env.TargetDotenv:
		return renderDotenv(out, selected.filter(items))
	case env.TargetKubernetes:
		return renderK8s(out, items, names, selected)
	default:
		key := options.Key
		if key == "" {
			key = "config.json"
			if selected.secrets && !selected.config {
				key = "secrets.json"
			}
		}
		return renderK8sBundle(out, items, names, key, selected)
	}
}

// parseSelection maps an Only value to the selected slices.
func parseSelection(only string) (selection, error) {
	switch only {
	case "", env.OnlyAll:
		return selection{secrets: true, config: true}, nil
	case env.OnlySecrets:
		return selection{secrets: true}, nil
	case env.OnlyConfig:
		return selection{config: true}, nil
	default:
		return selection{}, fmt.Errorf(
			"%w: only %q (want %q or %q)",
			ErrInvalidOption, only, env.OnlySecrets, env.OnlyConfig,
		)
	}
}

// kubernetes reports whether target renders a Kubernetes resource.
func kubernetes(target env.Target) bool {
	return target == env.TargetKubernetes || target == env.TargetKubernetesBundle
}

// filter keeps the selected slices of items.
func (s selection) filter(items []item) []item {
	out := make([]item, 0, len(items))
	for _, it := range items {
		if (it.secret && s.secrets) || (!it.secret && s.config) {
			out = append(out, it)
		}
	}
	return out
}

// partition keeps items whose secret flag equals secret.
func partition(items []item, secret bool) []item {
	out := make([]item, 0, len(items))
	for _, it := range items {
		if it.secret == secret {
			out = append(out, it)
		}
	}
	return out
}
