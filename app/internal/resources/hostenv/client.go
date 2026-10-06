package hostenv

import (
	"maps"
	"os"
	"strings"
)

// Client provides access to a point-in-time snapshot of the host process environment.
type Client struct {
	snapshot map[string]string
}

// New constructs a host environment client by capturing the process environment.
func New() *Client {
	environ := os.Environ()
	snapshot := make(map[string]string, len(environ))
	for _, entry := range environ {
		if name, value, found := strings.Cut(entry, "="); found {
			snapshot[name] = value
		}
	}
	return &Client{snapshot: snapshot}
}

// Get returns the value of the named variable from the snapshot and whether it is set.
func (c *Client) Get(name string) (string, bool) {
	value, ok := c.snapshot[name]
	return value, ok
}

// All returns an isolated copy of the captured environment snapshot.
func (c *Client) All() map[string]string {
	return maps.Clone(c.snapshot)
}
