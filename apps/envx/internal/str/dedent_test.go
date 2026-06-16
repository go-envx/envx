package str

import "testing"

// -------------------------------------------------------------------------------------
// TestDedent verifies that Dedent strips the common leading whitespace from
// all non-empty lines, expands tabs to spaces, and trims leading/trailing
// blank lines — making Go backtick-quoted strings safe for tab-sensitive
// formats like YAML.
func TestDedent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		indent int
		want   string
	}{
		{
			name:  "no indentation",
			input: "line1\nline2\n",
			want:  "line1\nline2",
		},
		{
			name:  "leading newline trimmed",
			input: "\nline1\nline2\n",
			want:  "line1\nline2",
		},
		{
			name:  "uniform indentation removed",
			input: "\n    line1\n    line2\n",
			want:  "line1\nline2",
		},
		{
			name:  "mixed indentation uses minimum",
			input: "\n    line1\n      line2\n    line3\n",
			want:  "line1\n  line2\nline3",
		},
		{
			name:  "blank lines preserved in middle",
			input: "\n    line1\n\n    line2\n",
			want:  "line1\n\nline2",
		},
		{
			name:  "tabs expanded to spaces",
			input: "\n\t\tline1\n\t\t  line2\n",
			want:  "line1\n  line2",
		},
		{
			name: "backtick style yaml",
			input: `
				environments: [dev]
				projects:
				  app:
				    path: apps/app/env
			`,
			want: "environments: [dev]\nprojects:\n  app:\n    path: apps/app/env",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "\n   \n   \n",
			want:  "",
		},
		{
			name:   "preserve indent adds spaces to non-empty lines",
			input:  "\n    line1\n    line2\n",
			indent: 2,
			want:   "  line1\n  line2",
		},
		{
			name:   "preserve indent keeps blank lines empty",
			input:  "\n    line1\n\n    line2\n",
			indent: 4,
			want:   "    line1\n\n    line2",
		},
		{
			name:   "preserve indent with mixed indentation",
			input:  "\n    line1\n      line2\n    line3\n",
			indent: 2,
			want:   "  line1\n    line2\n  line3",
		},
		{
			name:   "preserve indent of zero is a no-op",
			input:  "\n    line1\n    line2\n",
			indent: 0,
			want:   "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got string
			if tt.indent > 0 {
				got = Dedent(tt.input, tt.indent)
			} else {
				got = Dedent(tt.input)
			}
			if got != tt.want {
				t.Errorf("Dedent():\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
