package process

import (
	"regexp"
	"slices"
	"strings"
)

// baseEnv lists the variables a CLI needs on any platform to locate its home,
// configuration, cache, and runtime directories, and to reach the network.
var baseEnv = []string{
	"PATH",
	"HOME",
	"TMPDIR",
	"XDG_CONFIG_HOME",
	"XDG_CACHE_HOME",
	"XDG_RUNTIME_DIR",
	"USERPROFILE",
	"APPDATA",
	"LOCALAPPDATA",
	"SYSTEMROOT",
	"TEMP",
	"TMP",
	"HTTPS_PROXY",
	"HTTP_PROXY",
	"NO_PROXY",
	"SSL_CERT_FILE",
	"SSL_CERT_DIR",
}

// MinimalEnv selects from environ the platform base variables, matched
// case-insensitively, plus every variable whose name matches include. A nil
// include selects the base variables only.
func MinimalEnv(environ map[string]string, include *regexp.Regexp) map[string]string {
	env := make(map[string]string)
	for name, val := range environ {
		isBase := slices.ContainsFunc(baseEnv, func(base string) bool {
			return strings.EqualFold(name, base)
		})
		if isBase || include != nil && include.MatchString(name) {
			env[name] = val
		}
	}
	return env
}
