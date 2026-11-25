package app

import (
	"github.com/go-envx/envx/app/internal/core/external"
	"github.com/go-envx/envx/app/internal/core/validate"
)

// NewValidateService assembles the validation service for the workspace at
// configPath. The environment, secrets, and provider adapters are assembled
// only when a selected check needs them.
func (f Factory) NewValidateService(configPath string) (*validate.Service, error) {
	o, err := f.loadWorkspaceOptions(configPath)
	if err != nil {
		return nil, err
	}
	return validate.New(o.workspace, validate.Dependencies{
		OpenEnv: func() (validate.Explainer, error) {
			service, err := envService(o)
			if err != nil {
				return nil, err
			}
			return service, nil
		},
		OpenSecrets: func() (validate.Inspector, error) {
			service, err := secretsService(o)
			if err != nil {
				return nil, err
			}
			return service, nil
		},
		OpenProviders: func() (validate.ProviderInspector, error) {
			providers := newProviders(o.workspace.Secrets.Providers, o.hostEnv)
			return external.NewInspector(providers), nil
		},
	})
}
