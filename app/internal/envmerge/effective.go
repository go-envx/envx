package envmerge

// osSource is the sentinel provenance File for a value taken from the OS
// environment rather than a namespace file. It is not a real path; the explain
// presenter shows it verbatim.
const osSource = "OS environment"

// opaqueLeaf wraps an OS-environment value as a scalar leaf that bypasses
// reference resolution and substitution. OS values are opaque: a value that
// happens to look like a reference is never dereferenced.
func opaqueLeaf(value string) leafValue {
	return leafValue{items: []string{value}, opaque: true}
}

// osOriginSource builds the provenance Source for an OS-environment value.
func osOriginSource(key string) Source {
	return Source{File: osSource, Key: key}
}

// applyOSEnvironment overlays the injected OS-environment snapshot onto the
// merged state, implementing source selection independently of reveal. For a key
// defined by both a namespace and the OS the OS value wins by default and the
// namespace source is shadowed; under overload the namespace value wins and the
// OS source is recorded as shadowed instead, so the override stays visible in
// provenance. When unionOSKeys is set, OS-only keys are added as opaque winners
// so a materialized child receives a complete environment; get, explain, and diff
// leave them out because they enumerate namespace keys only.
func (m *Manager) applyOSEnvironment(state *mergeState, unionOSKeys bool) {
	overload := m.params.Settings.Overload
	for key, osValue := range m.params.OSEnvironment {
		_, isNamespace := state.values[key]
		switch {
		case isNamespace && !overload:
			state.values[key] = opaqueLeaf(osValue)
			state.origins[key] = osWins(state.origins[key], key)
		case isNamespace: // overload: the namespace value wins.
			state.origins[key] = osShadowed(state.origins[key], key)
		case unionOSKeys:
			state.values[key] = opaqueLeaf(osValue)
			state.origins[key] = Origin{Winner: osOriginSource(key)}
		}
	}
}

// osWins makes the OS environment the winning source for a key, demoting the
// prior namespace origin — its shadows first, then its former winner — behind the
// new OS winner so Shadowed still reads oldest-to-newest.
func osWins(prev Origin, key string) Origin {
	shadowed := make([]Source, 0, len(prev.Shadowed)+1)
	shadowed = append(shadowed, prev.Shadowed...)
	shadowed = append(shadowed, prev.Winner)
	return Origin{Winner: osOriginSource(key), Shadowed: shadowed}
}

// osShadowed keeps the namespace winner but records the OS environment as the
// newest shadowed source, so overload's precedence stays visible in provenance.
func osShadowed(prev Origin, key string) Origin {
	shadowed := make([]Source, 0, len(prev.Shadowed)+1)
	shadowed = append(shadowed, prev.Shadowed...)
	shadowed = append(shadowed, osOriginSource(key))
	return Origin{Winner: prev.Winner, Shadowed: shadowed}
}
