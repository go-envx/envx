package cli

import (
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/features/scaffold"
)

// TestSummary verifies the scaffold summary lists the written files and the
// first command to try, without a trailing newline so the printer owns it.
func TestSummary(t *testing.T) {
	t.Parallel()

	out := summary(summaryParams{
		Template:  scaffold.QuickStartTemplate,
		TargetDir: "workspace",
		Written:   []string{"workspace/envx.yaml"},
	})

	for _, want := range []string{
		"Scaffolded quick-start into workspace/ (1 files):",
		"  workspace/envx.yaml",
		"Try it:",
		"  cd workspace",
		"  envx get api-service DATABASE_HOST",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.HasSuffix(out, "\n") {
		t.Errorf("summary should not end with a newline:\n%q", out)
	}
}
