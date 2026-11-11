package process

import (
	"maps"
	"regexp"
	"testing"
)

func TestMinimalEnv(t *testing.T) {
	t.Parallel()

	environ := map[string]string{
		"Path":                     "/bin",
		"HOME":                     "/home/u",
		"https_proxy":              "http://proxy:3128",
		"OP_SERVICE_ACCOUNT_TOKEN": "ops_x",
		"op_session_acct":          "s",
		"AWS_SECRET_ACCESS_KEY":    "leak",
	}
	base := map[string]string{
		"Path":        "/bin",
		"HOME":        "/home/u",
		"https_proxy": "http://proxy:3128",
	}
	withOP := maps.Clone(base)
	withOP["OP_SERVICE_ACCOUNT_TOKEN"] = "ops_x"
	withOP["op_session_acct"] = "s"

	tests := map[string]struct {
		include *regexp.Regexp
		want    map[string]string
	}{
		"base only": {want: base},
		"include":   {include: regexp.MustCompile(`(?i)^OP_`), want: withOP},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := MinimalEnv(environ, tt.include); !maps.Equal(got, tt.want) {
				t.Errorf("MinimalEnv = %v, want %v", got, tt.want)
			}
		})
	}
}
