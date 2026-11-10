package testutil_test

import (
	"os"
	"testing"

	"github.com/go-envx/envx/app/internal/testutil"
)

// TestUnsetEnvPrefix verifies only variables with the prefix are removed.
func TestUnsetEnvPrefix(t *testing.T) {
	t.Setenv("TESTUTIL_PREFIX_A", "a")
	t.Setenv("OTHER_TESTUTIL_PREFIX", "b")

	testutil.UnsetEnvPrefix("TESTUTIL_PREFIX_")

	if _, ok := os.LookupEnv("TESTUTIL_PREFIX_A"); ok {
		t.Error("prefixed variable was not removed")
	}
	if _, ok := os.LookupEnv("OTHER_TESTUTIL_PREFIX"); !ok {
		t.Error("unprefixed variable was removed")
	}
}
