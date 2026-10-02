package env

// Manager is a deprecated alias for Service.
type Manager = Service

// Params is a deprecated alias for ServiceParams.
type Params = ServiceParams

// Explanation is a deprecated alias for ExplainResult.
type Explanation = ExplainResult

// Entry is a deprecated alias for GetResult.
type Entry = GetResult

// New is a deprecated alias for NewService.
//
//nolint:gocritic // Deprecated alias matches original signature.
func New(params ServiceParams) (*Service, error) {
	return NewService(params)
}
