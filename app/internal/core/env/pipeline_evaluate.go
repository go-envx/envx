package env

import (
	"slices"
	"strings"

	"github.com/go-envx/envx/app/internal/core/diagnostic"
	"github.com/go-envx/envx/app/internal/core/value"
)

// evaluator diagnoses winners of one merged environment without aborting on a
// per-key failure. Under masking, only references are materialized.
type evaluator struct {
	// merged is the merged environment.
	merged *mergedEnv
	// resolver dereferences values; nil is identity.
	resolver ValueResolver
	// op is the operation context.
	op operation
	// reveal retains materialized plaintext in evaluations.
	reveal bool
	// engine composes substitutions over merged.
	engine *substituter
	// leaves memoizes per-leaf evaluations.
	leaves map[string]value.Evaluation
	// leafFindings memoizes every distinct per-item finding of a leaf.
	leafFindings map[string][]diagnostic.Finding
	// secrets memoizes secret derivation.
	secrets map[string]bool
}

// newEvaluator builds an evaluator over one merged environment.
func newEvaluator(
	merged *mergedEnv, resolver ValueResolver, op operation, reveal bool,
) *evaluator {
	return &evaluator{
		merged:       merged,
		resolver:     resolver,
		op:           op,
		reveal:       reveal,
		engine:       newSubstituter(op, merged.symbols(resolver, op)),
		leaves:       make(map[string]value.Evaluation),
		leafFindings: make(map[string][]diagnostic.Finding),
		secrets:      make(map[string]bool),
	}
}

// entries evaluates keys in order.
func (e *evaluator) entries(keys []string) []Entry {
	out := make([]Entry, 0, len(keys))
	for _, key := range keys {
		out = append(out, e.entry(key))
	}
	return out
}

// entry evaluates one winning key.
func (e *evaluator) entry(key string) Entry {
	leaf := e.merged.values[key]
	eval, findings := e.evaluate(key, leaf)
	findings = slices.Clone(findings)
	for i := range findings {
		findings[i] = e.locate(findings[i], key)
	}
	if !eval.Diagnostic.IsZero() {
		eval.Diagnostic = e.locate(eval.Diagnostic, key)
	}
	var items []string
	raw := ""
	if !leaf.opaque {
		items = slices.Clone(leaf.items)
		raw = literalOf(leaf, e.op.settings.delimiter)
	} else if replaced, ok := e.merged.overridden[key]; ok {
		items = slices.Clone(replaced.items)
	}
	origin := e.merged.origins[key]
	origin.Shadowed = slices.Clone(origin.Shadowed)
	return Entry{
		Key:            key,
		Raw:            raw,
		Items:          items,
		Evaluation:     eval,
		Findings:       findings,
		Origin:         origin,
		Secret:         e.secret(key),
		DeclaredInBase: e.merged.declaredInBase[key],
	}
}

// locate stamps a finding with the operation's project and environment and key.
func (e *evaluator) locate(found diagnostic.Finding, key string) diagnostic.Finding {
	found.Project = e.op.project
	found.Environment = e.op.environment
	found.Key = key
	return found
}

// evaluate classifies one winner and returns its distinct findings. Host values
// are opaque, values touched by substitution go through the engine, and others
// are evaluated per item.
func (e *evaluator) evaluate(
	key string, leaf leafValue,
) (value.Evaluation, []diagnostic.Finding) {
	if leaf.opaque {
		eval := value.Evaluation{Kind: value.Config}
		if e.reveal {
			eval.Value = leaf.items[0]
			eval.Resolved = true
		}
		return eval, nil
	}
	literal := literalOf(leaf, e.op.settings.delimiter)
	refs := len(e.op.grammar.References(literal)) > 0
	if refs || e.op.grammar.HasEscape(literal) {
		eval := e.substitution(key, refs)
		if eval.Diagnostic.IsZero() {
			return eval, nil
		}
		return eval, []diagnostic.Finding{eval.Diagnostic}
	}
	return e.leaf(key), e.leafFindings[key]
}

// substitution evaluates a value through the engine. Only a live reference
// makes it a variable; the composed value is retained only under reveal.
func (e *evaluator) substitution(key string, variable bool) value.Evaluation {
	eval := value.Evaluation{Kind: value.Config}
	if variable {
		eval.Kind = value.Variable
	}
	composed, err := e.engine.resolve(key)
	if err != nil {
		code := substitutionCode(err)
		message := "references an undefined variable"
		if code == diagnostic.CircularVariableReference {
			message = "reference cycle detected"
		}
		eval.Diagnostic = e.finding(code, message)
		return eval
	}
	if e.reveal {
		eval.Value = composed
		eval.Resolved = true
	}
	return eval
}

// leaf evaluates each item of a leaf through the resolver, keeping the worst
// finding and joining item values. It is resolved only when every item is.
func (e *evaluator) leaf(key string) value.Evaluation {
	if eval, ok := e.leaves[key]; ok {
		return eval
	}
	leaf := e.merged.values[key]
	eval := value.Evaluation{Kind: value.Config, Resolved: true}
	var findings []diagnostic.Finding
	items := make([]string, len(leaf.items))
	for i, item := range leaf.items {
		out := value.Evaluation{Kind: value.Config, Value: item, Resolved: true}
		if e.resolver != nil {
			out = e.resolver.Evaluate(item, e.op.environment)
		}
		if out.Kind == value.Secret {
			eval.Kind = value.Secret
		}
		if out.Diagnostic.Outranks(eval.Diagnostic) {
			eval.Diagnostic = out.Diagnostic
		}
		if !out.Diagnostic.IsZero() && !slices.Contains(findings, out.Diagnostic) {
			findings = append(findings, out.Diagnostic)
		}
		items[i] = out.Value
		eval.Resolved = eval.Resolved && out.Resolved
	}
	eval.Value = strings.Join(items, e.op.settings.delimiter)
	// A resolved list must still render, as materialization requires.
	if eval.Resolved && leaf.list {
		resolved := leafValue{items: items, list: true}
		path := e.merged.origins[key].Winner.Key
		if _, err := renderLeaf(resolved, path, e.op.settings.delimiter); err != nil {
			eval.Value, eval.Resolved = "", false
		}
	}
	e.leaves[key] = eval
	e.leafFindings[key] = findings
	return eval
}

// secret reports whether key derives from a secret, directly or through the
// namespace variables it references. A host value that overrides a
// secret-derived namespace value is treated as secret.
func (e *evaluator) secret(key string) bool {
	derived, _ := e.derives(key, make(map[string]bool))
	return derived
}

// derives reports whether key derives from a secret and whether a negative
// answer is complete. A traversal cut short by a cycle is not cached as false,
// since another starting key may reach a secret through the cut path.
func (e *evaluator) derives(
	key string, visiting map[string]bool,
) (derived, complete bool) {
	if derived, ok := e.secrets[key]; ok {
		return derived, true
	}
	leaf, ok := e.merged.values[key]
	if !ok {
		return false, true
	}
	if visiting[key] {
		return false, false
	}
	if leaf.opaque {
		if leaf, ok = e.merged.overridden[key]; !ok {
			return false, true
		}
		derived = e.hasSecretItem(leaf)
	} else {
		derived = e.leaf(key).Kind == value.Secret
	}
	complete = true
	visiting[key] = true
	literal := literalOf(leaf, e.op.settings.delimiter)
	for _, name := range e.op.grammar.References(literal) {
		if derived {
			break
		}
		// Declared keys already hold the value a reference selects: host values
		// replace them unless overload keeps the namespace value.
		if _, declared := e.merged.values[name]; !declared {
			continue
		}
		var done bool
		derived, done = e.derives(name, visiting)
		complete = complete && done
	}
	delete(visiting, key)
	if derived || complete {
		e.secrets[key] = derived
		return derived, true
	}
	return false, false
}

// hasSecretItem reports whether any item of leaf is a secret reference.
func (e *evaluator) hasSecretItem(leaf leafValue) bool {
	if e.resolver == nil {
		return false
	}
	for _, item := range leaf.items {
		if e.resolver.Evaluate(item, e.op.environment).Kind == value.Secret {
			return true
		}
	}
	return false
}

// finding builds a finding at the code's built-in severity.
func (e *evaluator) finding(code diagnostic.Code, message string) diagnostic.Finding {
	def, _ := diagnostic.Lookup(code)
	return diagnostic.Finding{Code: code, Level: def.Level, Message: message}
}
