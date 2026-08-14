package envmerge

// Build is the legacy compatibility entry point retained while callers migrate to
// Manager operations. It constructs a Manager, resolves and validates the default
// environment, merges it, and returns a Result that may carry per-key resolution
// failures so single-key consumers can ignore unrelated failures while whole-
// environment consumers fail loudly via Result.Verify. It uses Params.ValueResolver
// directly; Manager operations obtain a resolver from Params.ResolverFactory
// instead.
func Build(p Params) (*Result, error) {
	manager, err := New(p)
	if err != nil {
		return nil, err
	}

	environment, err := manager.normalizeEnvironment("")
	if err != nil {
		return nil, err
	}

	state, err := manager.merge(environment)
	if err != nil {
		return nil, err
	}

	result := materialize(
		state, manager.params.Settings, manager.params.ValueResolver, environment,
	)
	return &Result{resolved: resolved{
		values:  result.values,
		origins: result.origins,
		errs:    result.errs,
	}}, nil
}
