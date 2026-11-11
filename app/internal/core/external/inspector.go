package external

// Inspector checks enabled providers without contacting them.
type Inspector struct {
	// providers holds the enabled providers by scheme.
	providers map[string]Provider
}

// NewInspector constructs an inspector over the enabled providers.
func NewInspector(providers []Registration) *Inspector {
	byScheme := make(map[string]Provider, len(providers))
	for _, reg := range providers {
		if reg.Provider != nil {
			byScheme[reg.Provider.Scheme()] = reg.Provider
		}
	}
	return &Inspector{providers: byScheme}
}

// CheckAvailable returns why the enabled provider for scheme cannot be called.
// It returns nil when the provider looks usable or cannot tell offline, and when
// scheme is not enabled.
func (i *Inspector) CheckAvailable(scheme string) error {
	checker, ok := i.providers[scheme].(AvailabilityChecker)
	if !ok {
		return nil
	}
	return checker.CheckAvailable()
}
