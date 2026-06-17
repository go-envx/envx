package app

import (
	"fmt"
	"strings"
)

// -------------------------------------------------------------------------------------
// Get resolves the merged environment for a project and returns the value of
// the specified key. The key is matched case-insensitively (uppercased).
func (a *App) Get(in PipelineInput, key string) (string, error) {
	_, _, result, err := a.ResolvePipeline(in)
	if err != nil {
		return "", err
	}

	lookup := strings.ToUpper(key)
	val, ok := result.Env[lookup]
	if !ok {
		available := make([]string, 0, len(result.Env))
		for k := range result.Env {
			available = append(available, k)
		}
		return "", fmt.Errorf("key %q not found (available: %v)", lookup, available)
	}
	return val, nil
}
