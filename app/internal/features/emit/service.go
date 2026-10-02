package emit

import "os"

// ServiceParams provides dependencies to the target serialization service. The
// service is stateless: the destination writer arrives with each Render.
type ServiceParams struct{}

// Service renders resolved environment entries into delivery target formats.
type Service struct{}

// NewService constructs a target serialization domain service.
func NewService(ServiceParams) (*Service, error) {
	return &Service{}, nil
}

// Render writes the entries to params.Writer (os.Stdout when nil) in the
// target's shape,
// restricted to the selected slice: IncludeSecrets keeps the secret-derived
// values and IncludeConfig the plain ones (at least one must be set). json and
// dotenv render the selected values as a single document. k8s renders a Secret
// for the secrets and a ConfigMap for the config, each value its own key;
// k8s-bundle renders the slice as a single Key holding one blob. Entries render
// in sorted key order so output is deterministic.
func (s *Service) Render(params RenderParams) error {
	if !params.IncludeSecrets && !params.IncludeConfig {
		return ErrNoSliceSelected
	}
	w := params.Writer
	if w == nil {
		w = os.Stdout
	}
	sorted := sortedEntries(params.Entries)
	switch params.Target {
	case TargetJSON:
		return renderJSON(w, sliceEntries(sorted, params))
	case TargetDotenv:
		return renderDotenv(w, sliceEntries(sorted, params))
	case TargetK8s:
		return renderK8s(w, sorted, params)
	case TargetK8sBundle:
		return renderK8sBundle(w, sorted, params)
	default:
		return unknownTargetError(string(params.Target))
	}
}
