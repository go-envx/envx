package testutil

import (
	"os"
	"strings"
)

// UnsetEnvPrefix removes every process environment variable whose name starts
// with prefix. Call it from TestMain so ambient settings cannot reach parallel
// tests, which cannot use t.Setenv.
func UnsetEnvPrefix(prefix string) {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, prefix) {
			_ = os.Unsetenv(name)
		}
	}
}
