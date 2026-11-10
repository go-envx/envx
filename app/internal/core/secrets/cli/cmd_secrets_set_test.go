package cli

import (
	"strings"
	"testing"
)

// TestSecretsSetCommand verifies the plaintext comes from the argument, or from
// stdin when omitted, and the summary names the secret without its value.
func TestSecretsSetCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// name labels the case.
		name string
		// stdin is the command's standard input.
		stdin string
		// args are the arguments after "secrets set".
		args []string
		// want is the stored plaintext.
		want string
	}{
		{
			name: "argument",
			args: []string{"production", "token", "from-arg"},
			want: "from-arg",
		},
		{
			name:  "stdin",
			stdin: "from-stdin\n",
			args:  []string{"production", "token"},
			want:  "from-stdin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := seededFactory(t)
			args := append([]string{"secrets", "set"}, tt.args...)
			stdout, _, err := execute(t, factory, tt.stdin, args...)
			if err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			got, err := factory.service(t).GetSecret("production", "token")
			if err != nil || got.Value != tt.want {
				t.Errorf("stored = %q, %v; want %q", got.Value, err, tt.want)
			}
			summary := `Stored secret "token" in group "production"`
			if !strings.Contains(stdout, summary) ||
				strings.Contains(stdout, tt.want) {
				t.Errorf("stdout = %q, want a summary without the value", stdout)
			}
		})
	}
}

// TestSecretsSetCommandEmptyStdin verifies a missing plaintext fails without
// storing anything.
func TestSecretsSetCommandEmptyStdin(t *testing.T) {
	t.Parallel()

	factory := seededFactory(t)
	_, _, err := execute(t, factory, "", "secrets", "set", "production", "token")
	if err == nil {
		t.Fatal("expected an error for empty stdin")
	}
	if factory.stored("production", "token") != "" {
		t.Error("a value was stored despite the missing plaintext")
	}
}
