package infra

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/schema"
)

// SchemaDocsURL is the manifest schema reference pointed to when a key is
// rejected, so a user gets the authoritative key list without the error naming
// internal Go types.
const SchemaDocsURL = "https://go-envx.github.io/envx/configuration/schema/"

// suggestionThreshold is the largest edit distance at which a rejected key is
// still close enough to a valid one to offer as a "did you mean" correction.
// Three keeps the common typo and the snake/kebab rename in range without
// suggesting unrelated keys.
const suggestionThreshold = 3

// continuationIndent aligns a message's follow-up lines under its first line,
// which the printer prefixes with the "ERROR:" label.
const continuationIndent = "\n   "

// strictDecode decodes data into a schema.Manifest with unknown-field rejection
// so a removed, renamed, or misspelled manifest key fails loudly rather than
// being silently dropped. yaml.Node.Decode has no strict mode, so a dedicated
// decoder runs over the raw bytes; the caller keeps the node round-trip only for
// indentation detection. A rejected key is reported in manifest-domain terms —
// the key, its line, the nearest valid key, and the schema docs — rather than
// leaking yaml's internal type names.
func strictDecode(data []byte) (schema.Manifest, error) {
	var manifest schema.Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&manifest); err != nil {
		return schema.Manifest{}, decodeError(err)
	}
	return manifest, nil
}

// unknownFieldPattern matches yaml.v3's "field <name> not found in type <type>"
// diagnostic so the offending key, its line, and its owning struct can be
// recovered and reported in domain terms.
var unknownFieldPattern = regexp.MustCompile(
	`^line (\d+): field (\S+) not found in type (\S+)$`,
)

// fieldContext describes one decodable manifest struct: a human label for the
// block it represents and the type whose yaml tags enumerate that block's valid
// keys for the "did you mean" suggestion.
type fieldContext struct {
	label string
	typ   reflect.Type
}

// manifestFieldContexts maps each decodable struct's yaml type name (as it
// appears in yaml.v3's diagnostic) to the label and type used to describe and
// correct a rejected key within it.
var manifestFieldContexts = map[string]fieldContext{
	"schema.Manifest": {label: "manifest key", typ: reflect.TypeOf(schema.Manifest{})},
	"schema.Settings": {label: "setting", typ: reflect.TypeOf(schema.Settings{})},
	"schema.SecretsConfig": {
		label: "secrets setting",
		typ:   reflect.TypeOf(schema.SecretsConfig{}),
	},
	"schema.Project": {label: "project key", typ: reflect.TypeOf(schema.Project{})},
}

// decodeError translates a yaml decode failure into a manifest-domain error.
// Unknown-key rejections are rewritten to name the key, its line, and the
// nearest valid key, with a single docs pointer appended; any other decode
// failure is wrapped unchanged.
func decodeError(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	lines := make([]string, 0, len(typeErr.Errors)+1)
	sawUnknownField := false
	for _, raw := range typeErr.Errors {
		described, ok := describeUnknownField(raw)
		if !ok {
			// A non-key decode failure (such as a type mismatch) carries no Go
			// type name worth hiding, so it is surfaced as reported.
			lines = append(lines, raw)
			continue
		}
		sawUnknownField = true
		lines = append(lines, described...)
	}
	if sawUnknownField {
		lines = append(lines, "See "+SchemaDocsURL)
	}

	return fmt.Errorf("manifest: %s", strings.Join(lines, continuationIndent))
}

// describeUnknownField rewrites one "field not found" diagnostic into a labeled
// message plus an optional nearest-key suggestion, returning ok=false when the
// diagnostic is not an unknown-key rejection.
func describeUnknownField(raw string) ([]string, bool) {
	match := unknownFieldPattern.FindStringSubmatch(raw)
	if match == nil {
		return nil, false
	}
	lineNo, field, typeName := match[1], match[2], match[3]

	ctx, known := manifestFieldContexts[typeName]
	label := "manifest key"
	if known {
		label = ctx.label
	}

	out := []string{fmt.Sprintf("unknown %s %q (line %s)", label, field, lineNo)}
	if known {
		if suggestion, found := nearestKey(field, validKeys(ctx.typ)); found {
			out = append(out, fmt.Sprintf("Did you mean %q?", suggestion))
		}
	}
	return out, true
}

// validKeys returns the yaml key names declared by a struct's fields, skipping
// unnamed and omitted tags so the suggestion set matches what the manifest
// actually accepts.
func validKeys(typ reflect.Type) []string {
	keys := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// nearestKey returns the candidate closest to field by edit distance, and
// whether any candidate was within suggestionThreshold.
func nearestKey(field string, candidates []string) (string, bool) {
	best := ""
	bestDist := suggestionThreshold + 1
	for _, candidate := range candidates {
		if dist := levenshtein(field, candidate); dist < bestDist {
			bestDist, best = dist, candidate
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}

// levenshtein returns the edit distance between a and b using a single rolling
// row, comparing by rune so multi-byte keys are measured correctly.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr := make([]int, len(rb)+1)
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(rb)]
}
