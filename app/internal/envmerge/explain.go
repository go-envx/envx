package envmerge

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-envx/envx/app/internal/features/env/syntax"
	"github.com/go-envx/envx/app/internal/shared/status"
	"github.com/go-envx/envx/app/internal/shared/value"
	"github.com/go-envx/envx/app/internal/utils/severity"
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
	// Items is the winning leaf's raw, pre-resolution items, before joining or
	// dereferencing. It is nil for an opaque OS value, which is never
	// dereferenced, so a caller enumerating references never mistakes an OS value
	// for one. A workspace-wide validator reads it to discover which stored
	// secrets a reference actually uses.
	Items []string
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
	engine := m.newSubstituter(m.getSymbols(state, resolver, environment))

	delimiter := m.params.Settings.Delimiter
	entries := make([]ExplanationEntry, 0, len(keys))
	var summary ExplanationSummary
	for _, key := range keys {
		val := state.values[key]
		literal := literalValue(val, delimiter)
		resolution := diagnoseEntry(
			val, literal, key, diagnoser, engine, environment, delimiter,
			params.Reveal,
		)
		switch resolution.Severity {
		case SeverityError:
			summary.Errors++
		case SeverityWarning:
			summary.Warnings++
		case SeverityOK, severity.None:
		}
		entries = append(entries, ExplanationEntry{
			Key:        key,
			Literal:    literal,
			Items:      itemsOf(val),
			Origin:     state.origins[key],
			Resolution: resolution,
		})
	}

	return &Explanation{Entries: entries, Summary: summary}, nil
}

// itemsOf returns a copy of a leaf's raw pre-resolution items, or nil for an
// opaque OS value that is never dereferenced. Copying keeps the caller from
// aliasing the manager's merge state.
func itemsOf(leaf leafValue) []string {
	if leaf.opaque {
		return nil
	}
	items := make([]string, len(leaf.items))
	copy(items, leaf.items)
	return items
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

// asDiagnoser adapts an opened resolver into a ValueDiagnoser (or value.Evaluator).
// A nil resolver yields a nil diagnoser, which diagnoseLeaf treats as plain
// config-value identity. A resolver that does not also implement ValueDiagnoser is an
// operation error, so a reference is never silently classified as a plain config
// value.
func asDiagnoser(resolver ValueResolver) (ValueDiagnoser, error) {
	if resolver == nil {
		return nil, nil
	}
	if diagnoser, ok := resolver.(ValueDiagnoser); ok {
		return diagnoser, nil
	}
	if evaluator, ok := resolver.(value.Evaluator); ok {
		return evaluatorAdapter{evaluator: evaluator}, nil
	}
	return nil, fmt.Errorf("configured resolver does not support diagnosis")
}

type evaluatorAdapter struct {
	evaluator value.Evaluator
}

func (a evaluatorAdapter) Diagnose(raw, env string) Resolution {
	return a.evaluator.Evaluate(raw, env)
}

// diagnoseEntry classifies one winning value. A non-opaque value touched by the
// substitution stage — one carrying a {{ }} reference or a \{{ escape — is
// diagnosed through the engine so its revealed value matches run and get; every
// other value is diagnosed as a plain config value or secret reference.
func diagnoseEntry(
	leaf leafValue, literal, key string,
	diagnoser ValueDiagnoser, engine *syntax.Substituter,
	environment, delimiter string, reveal bool,
) Resolution {
	refs := engine.Grammar().HasReferences(literal)
	if !leaf.opaque && (refs || engine.Grammar().HasEscape(literal)) {
		return diagnoseSubstitution(engine, key, reveal, refs)
	}
	return diagnoseLeaf(leaf, diagnoser, environment, delimiter, reveal)
}

// diagnoseSubstitution classifies a substitution-stage value in dry-run mode. It
// reports resolvability through the engine's status pass without exposing the
// composed value, mapping a missing reference to UNRESOLVED_VARIABLE_REFERENCE
// and a cycle to CIRCULAR_VARIABLE_REFERENCE. An escape-only value references
// nothing, so it is a plain config value that always resolves; only a live
// reference marks it as a variable substitution. The composed value is
// materialized and retained only under reveal, so masked diagnosis never leaks
// it.
func diagnoseSubstitution(
	engine *syntax.Substituter, key string, reveal, variable bool,
) Resolution {
	kind := KindConfigValue
	if variable {
		kind = KindVariableSubstitution
	}
	resolution := Resolution{
		Kind:     kind,
		Severity: SeverityOK,
		Status:   status.OK,
		Code:     status.OK,
	}
	switch engine.Status(key) {
	case syntax.ResolutionCircular:
		resolution.Severity = SeverityError
		resolution.Status = status.CircularVariableReference
		resolution.Code = status.CircularVariableReference
		resolution.Message = "reference cycle detected"
	case syntax.ResolutionUnresolved:
		resolution.Severity = SeverityError
		resolution.Status = status.UnresolvedVariableReference
		resolution.Code = status.UnresolvedVariableReference
		resolution.Message = "references an undefined variable"
	case syntax.ResolutionOK:
		if reveal {
			// status already composed the value successfully; resolve returns the
			// cached result, so no work is repeated and no error is possible here.
			composed, _ := engine.Resolve(key)
			resolution.Value = composed
			resolution.Resolved = composed
			resolution.IsResolved = true
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
	leaf leafValue, diagnoser ValueDiagnoser, environment, delimiter string,
	reveal bool,
) Resolution {
	// An opaque OS value is a plain config value that resolves to itself; its
	// plaintext is retained only under reveal, mirroring a plain config value.
	if leaf.opaque {
		resolution := Resolution{
			Kind:     KindConfigValue,
			Severity: SeverityOK,
			Status:   status.OK,
			Code:     status.OK,
		}
		if reveal {
			lit := literalValue(leaf, delimiter)
			resolution.Value = lit
			resolution.Resolved = lit
			resolution.IsResolved = true
			resolution.HasResolved = true
		}
		return resolution
	}

	if diagnoser == nil {
		return Resolution{
			Kind:     KindConfigValue,
			Severity: SeverityOK,
			Status:   status.OK,
			Code:     status.OK,
		}
	}

	agg := Resolution{
		Kind:     KindConfigValue,
		Severity: SeverityOK,
		Status:   status.OK,
		Code:     status.OK,
	}
	resolvedItems := make([]string, len(leaf.items))
	allResolved := len(leaf.items) > 0
	for i, item := range leaf.items {
		outcome := diagnoser.Diagnose(item, environment)
		if outcome.Kind == KindSecretReference {
			agg.Kind = KindSecretReference
		}
		if severityRank(outcome.Severity) > severityRank(agg.Severity) {
			agg.Severity = outcome.Severity
			agg.Status = outcome.Status
			agg.Code = outcome.Code
			agg.Message = outcome.Message
		}
		if outcome.HasResolved || outcome.IsResolved {
			resolvedItems[i] = outcome.Resolved
			if resolvedItems[i] == "" {
				resolvedItems[i] = outcome.Value
			}
		} else {
			allResolved = false
		}
	}

	if allResolved {
		joined := strings.Join(resolvedItems, delimiter)
		agg.Value = joined
		agg.Resolved = joined
		agg.IsResolved = true
		agg.HasResolved = true
	}
	return agg
}

// severityRank orders severities so aggregation can select the worst outcome.
func severityRank(sev Severity) int {
	return sev.Rank()
}
