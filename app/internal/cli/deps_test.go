package cli

import (
	"github.com/go-envx/envx/app/internal/core/env"
	"github.com/go-envx/envx/app/internal/core/pack"
	"github.com/go-envx/envx/app/internal/core/scaffold"
	"github.com/go-envx/envx/app/internal/core/secrets"
	"github.com/go-envx/envx/app/internal/core/validate"
)

var _ appFactory = fakeFactory{}

// fakeFactory returns preassembled services and records nothing.
type fakeFactory struct {
	// env is returned by NewEnvService.
	env *env.Service
	// secrets is returned by NewSecretsService.
	secrets *secrets.Service
	// pack is returned by NewPackService.
	pack *pack.Service
	// validate is returned by NewValidateService.
	validate *validate.Service
	// scaffold is returned by NewScaffoldService.
	scaffold *scaffold.Service
	// err is returned by every workspace-dependent constructor.
	err error
}

func (f fakeFactory) NewEnvService(string) (*env.Service, error) {
	return f.env, f.err
}

func (f fakeFactory) NewSecretsService(string) (*secrets.Service, error) {
	return f.secrets, f.err
}

func (f fakeFactory) NewPackService(string) (*pack.Service, error) {
	return f.pack, f.err
}

func (f fakeFactory) NewValidateService(string) (*validate.Service, error) {
	return f.validate, f.err
}

func (f fakeFactory) NewScaffoldService() *scaffold.Service {
	return f.scaffold
}
