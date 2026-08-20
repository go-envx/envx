package envmerge

import (
	"fmt"
	"sort"
	"strings"
)

// Status codes for a diagnosed value. codeOK marks a successful resolution,
// while the substitution codes classify a dry-run substitution failure.
const (
	// codeOK is the status code for a value that resolved successfully.
	codeOK = "OK"
	// codeUnresolvedVariable marks a substitution with a missing internal or OS
	// reference.
	codeUnresolvedVariable = "UNRESOLVED_VARIABLE"
	// codeCircularReference marks a substitution with a reference cycle.
	codeCircularReference = "CIRCULAR_REFERENCE"
)

// ExplainParams selects all keys or one case-insensitive key from an environment
// and chooses the reveal policy for a diagnostic explanation.
type ExplainParams struct {
	// Key selects one case-insensitive key; an empty value explains every key.
	Key string
	// Environment overrides the configured default; an empty value uses it.
	Environment string
	// Reveal controls whether the opened resolver materializes plaintext into
	// each entry's Resolution.
	Reveal bool
}

// Explanation is a sorted diagnostic view of the selected winning values. It
// carries per-key literals, provenance, and non-fatal resolution status, and is
// never consumable as a process environment.
type Explanation struct {
	// Entries holds one diagnostic row per selected key, sorted by key.
	Entries []ExplanationEntry
	// Summary aggregates the resolution severities across the selected entries.
	Summary ExplanationSummary
}

// ExplanationEntry records one winning literal, its provenance, and its
// non-fatal resolution status.
type ExplanationEntry struct {
	// Key is the canonical uppercase env-var key.
	Key string
	// Literal is the pre-resolution winning value written in the source file.
	Literal string
	// Origin records the winning source and every source it shadowed.
	Origin Origin
	// Resolution is the non-fatal, dry-run outcome of diagnosing the value.
	Resolution Resolution
}

// ExplanationSummary aggregates resolution severities across the selected
// entries so a presenter can lead with a banner when resolution is incomplete.
type ExplanationSummary struct {
	// Errors is the number of entries at error severity.
	Errors int
	// Warnings is the number of entries at warning severity.
	Warnings int
}

// Severity returns the worst severity represented by the summary.
func (s ExplanationSummary) Severity() Severity {
	switch {
	case s.Errors > 0:
		return SeverityError
	case s.Warnings > 0:
		return SeverityWarning
	default:
		return SeverityOK
	}
}

// Explain loads the requested environment, selects one key or every winning key
// in sorted order, and diagnoses each selected value without aborting on a
// per-key resolution failure. Namespace, YAML, and flatten failures remain fatal
// because there is no valid merge to explain, while a per-value failure is
// carried by its Resolution. The fresh resolver receives the reveal policy, and
// diagnosis still attempts decryption when masked so status stays meaningful;
// plaintext is retained only when reveal is requested.
func (m *Manager) Explain(params ExplainParams) (*Explanation, error) {
	environment, err := m.normalizeEnvironment(params.Environment)
	if err != nil {
		return nil, err
	}

	state, err := m.merge(environment)
	if err != nil {
		return nil, err
	}

	// Apply OS source selection over the namespace keys so an override surfaces as
	// an "OS environment" source; OS-only keys are left out of the enumeration.
	m.applyOSEnvironment(state, false)

	keys, err := explainKeys(state, params.Key)
	if err != nil {
		return nil, err
	}

	resolver, err := m.openResolver(params.Reveal)
	if err != nil {
		return nil, err
	}
	diagnoser, err := asDiagnoser(resolver)
	if err != nil {
		return nil, err
	}
	engine := newSymbolSubstituter(
		m.getSymbols(state, resolver, environment),
		m.getenv(), m.params.Settings.Overload,
	)

	delimiter := m.params.Settings.Delimiter
	entries := make([]ExplanationEntry, 0, len(keys))
	var summary ExplanationSummary
	for _, key := range keys {
		value := state.values[key]
		literal := literalValue(value, delimiter)
		resolution := diagnoseEntry(
			value, literal, key, diagnoser, engine, environment, delimiter,
			params.Reveal,
		)
		switch resolution.Severity {
		case SeverityError:
			summary.Errors++
		case SeverityWarning:
			summary.Warnings++
		case SeverityOK:
		}
		entries = append(entries, ExplanationEntry{
			Key:        key,
			Literal:    literal,
			Origin:     state.origins[key],
			Resolution: resolution,
		})
	}

	return &Explanation{Entries: entries, Summary: summary}, nil
}

// explainKeys returns the keys to diagnose in sorted order: every winning key
// when key is empty, or the single normalized key otherwise. A requested key
// that is absent is an operation error.
func explainKeys(state *mergeState, key string) ([]string, error) {
	if key != "" {
		upper := strings.ToUpper(key)
		if _, ok := state.values[upper]; !ok {
			return nil, fmt.Errorf("key %q not found", upper)
		}
		return []string{upper}, nil
	}

	keys := make([]string, 0, len(state.values))
	for k := range state.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// asDiagnoser adapts an opened resolver into a ValueDiagnoser. A nil resolver
// yields a nil diagnoser, which diagnoseLeaf treats as plain config-value
// identity. A resolver that does not also implement ValueDiagnoser is an
// operation error, so a reference is never silently classified as a plain config
// value.
func asDiagnoser(resolver ValueResolver) (ValueDiagnoser, error) {
	if resolver == nil {
		return nil, nil
	}
	diagnoser, ok := resolver.(ValueDiagnoser)
	if !ok {
		return nil, fmt.Errorf("configured resolver does not support diagnosis")
	}
	return diagnoser, nil
}

// diagnoseEntry classifies one winning value. A non-opaque value touched by the
// substitution stage — one carrying a {{ }} reference or a \{{ escape — is
// diagnosed through the engine so its revealed value matches run and get; every
// other value is diagnosed as a plain config value or secret reference.
func diagnoseEntry(
	value leafValue, literal, key string,
	diagnoser ValueDiagnoser, engine *substituter,
	environment, delimiter string, reveal bool,
) Resolution {
	refs := hasReferences(literal)
	if !value.opaque && (refs || hasEscape(literal)) {
		return diagnoseSubstitution(engine, key, reveal, refs)
	}
	return diagnoseLeaf(value, diagnoser, environment, delimiter, reveal)
}

// diagnoseSubstitution classifies a substitution-stage value in dry-run mode. It
// reports resolvability through the engine's status pass without exposing the
// composed value, mapping a missing reference to UNRESOLVED_VARIABLE and a cycle
// to CIRCULAR_REFERENCE. An escape-only value references nothing, so it is a plain
// config value that always resolves; only a live reference marks it as a variable
// substitution. The composed value is materialized and retained only under
// reveal, so masked diagnosis never leaks it.
func diagnoseSubstitution(
	engine *substituter, key string, reveal, variable bool,
) Resolution {
	kind := KindConfigValue
	if variable {
		kind = KindVariableSubstitution
	}
	resolution := Resolution{Kind: kind, Severity: SeverityOK, Code: codeOK}
	switch engine.status(key) {
	case statusCircular:
		resolution.Severity = SeverityError
		resolution.Code = codeCircularReference
		resolution.Message = "reference cycle detected"
	case statusUnresolved:
		resolution.Severity = SeverityError
		resolution.Code = codeUnresolvedVariable
		resolution.Message = "references an undefined variable"
	case statusOK:
		if reveal {
			// status already composed the value successfully; resolve returns the
			// cached result, so no work is repeated and no error is possible here.
			composed, _ := engine.resolve(key)
			resolution.Resolved = composed
			resolution.HasResolved = true
		}
	}
	return resolution
}

// diagnoseLeaf classifies one winning leaf, aggregating a list's items into a
// single Resolution at the worst observed severity so a single failing item
// surfaces without revealing which one. A nil diagnoser treats every value as a
// plain config value. The resolved plaintext is populated only when every item
// materialized, and list items are rejoined with the delimiter.
func diagnoseLeaf(
	value leafValue, diagnoser ValueDiagnoser, environment, delimiter string,
	reveal bool,
) Resolution {
	// An opaque OS value is a plain config value that resolves to itself; its
	// plaintext is retained only under reveal, mirroring a plain config value.
	if value.opaque {
		resolution := Resolution{
			Kind: KindConfigValue, Severity: SeverityOK, Code: codeOK,
		}
		if reveal {
			resolution.Resolved = literalValue(value, delimiter)
			resolution.HasResolved = true
		}
		return resolution
	}

	if diagnoser == nil {
		return Resolution{Kind: KindConfigValue, Severity: SeverityOK, Code: codeOK}
	}

	agg := Resolution{Kind: KindConfigValue, Severity: SeverityOK, Code: codeOK}
	resolvedItems := make([]string, len(value.items))
	allResolved := len(value.items) > 0
	for i, item := range value.items {
		outcome := diagnoser.Diagnose(item, environment)
		if outcome.Kind == KindSecretReference {
			agg.Kind = KindSecretReference
		}
		if severityRank(outcome.Severity) > severityRank(agg.Severity) {
			agg.Severity = outcome.Severity
			agg.Code = outcome.Code
			agg.Message = outcome.Message
		}
		if outcome.HasResolved {
			resolvedItems[i] = outcome.Resolved
		} else {
			allResolved = false
		}
	}

	if allResolved {
		agg.Resolved = strings.Join(resolvedItems, delimiter)
		agg.HasResolved = true
	}
	return agg
}

// severityRank orders severities so aggregation can select the worst outcome.
func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 2
	case SeverityWarning:
		return 1
	default:
		return 0
	}
}
