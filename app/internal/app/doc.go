// Package app assembles feature services from a workspace manifest. Each
// constructor loads one workspace snapshot and wires only the dependencies its
// feature uses; no services are cached across calls.
package app
