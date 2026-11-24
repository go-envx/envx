package external

import (
	"fmt"

	"github.com/go-envx/envx/app/internal/lib/hostenv"
)

// HostValues returns the lookup's variable values by name. A read failure is
// reported as an unavailable provider.
func HostValues(hostEnv *hostenv.Lookup) (map[string]string, error) {
	vars, err := hostEnv.All()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}
	values := make(map[string]string, len(vars))
	for name, v := range vars {
		values[name] = v.Value
	}
	return values, nil
}
