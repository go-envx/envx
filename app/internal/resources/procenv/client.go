package procenv

import (
	"os"
	"strings"
)

// Params configures the process environment client.
type Params struct{}

// Client reads the current process environment.
type Client struct{}

// New constructs a process environment client.
func New(Params) (*Client, error) {
	return &Client{}, nil
}

// LookupEnv returns the value of the named variable and whether it is set.
func (*Client) LookupEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

// Environ snapshots the process environment as a name-to-value map.
func (*Client) Environ() map[string]string {
	environ := os.Environ()
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		if name, value, found := strings.Cut(entry, "="); found {
			out[name] = value
		}
	}
	return out
}
