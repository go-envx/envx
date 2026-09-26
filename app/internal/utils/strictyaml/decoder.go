package strictyaml

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-envx/envx/app/internal/utils/str"
)

// DefaultSuggestionThreshold is the default maximum edit distance for suggestions.
const DefaultSuggestionThreshold = 3

// unknownFieldPattern matches yaml.v3's "field <name> not found in type <type>"
// diagnostic so the offending key, its line, and its owning struct can be
// recovered and reported.
var unknownFieldPattern = regexp.MustCompile(
	`^line (\d+): field (.+) not found in type (\S+)$`,
)

// UnknownField represents a single rejected YAML field with diagnostic metadata.
type UnknownField struct {
	// Line is the 1-based line number where the unknown field appeared.
	Line int
	// Field is the raw key name found in the YAML document.
	Field string
	// TypeName is the Go struct type name reported by yaml.v3.
	TypeName string
	// Label is the human-readable block descriptor (e.g. "setting"), or "key" by default.
	Label string
	// Suggestion is the nearest valid key within the edit distance threshold, if any.
	Suggestion string
}

// DecodeError captures all structured diagnostics from a strict decode failure.
type DecodeError struct {
	// UnknownFields lists every rejected field with suggestions and line numbers.
	UnknownFields []UnknownField
	// OtherErrors captures any non-unknown-field decode errors (e.g. type mismatches).
	OtherErrors []string
}

// Error formats the decode failure with a clean default multi-line presentation.
func (e *DecodeError) Error() string {
	lines := make([]string, 0, len(e.UnknownFields)+len(e.OtherErrors))
	for _, f := range e.UnknownFields {
		msg := fmt.Sprintf("unknown %s %q (line %d)", f.Label, f.Field, f.Line)
		if f.Suggestion != "" {
			msg += fmt.Sprintf(": did you mean %q?", f.Suggestion)
		}
		lines = append(lines, msg)
	}
	lines = append(lines, e.OtherErrors...)
	return strings.Join(lines, "\n")
}

// HasUnknownFields reports whether any unknown fields were detected.
func (e *DecodeError) HasUnknownFields() bool {
	return len(e.UnknownFields) > 0
}

// FieldContext describes a struct schema for unknown-field error reporting.
type FieldContext struct {
	Label  string
	Schema reflect.Type
}

// Option configures a Decoder instance.
type Option func(*Decoder)

// WithSchema registers a schema struct type T and its human-readable label.
// T can be a struct or a pointer to a struct.
func WithSchema[T any](label string) Option {
	return func(d *Decoder) {
		schema := reflect.TypeFor[T]()
		for schema.Kind() == reflect.Pointer {
			schema = schema.Elem()
		}
		name := schema.Name()
		d.contexts[name] = FieldContext{
			Label:  label,
			Schema: schema,
		}
	}
}

// WithSuggestionThreshold sets the maximum edit distance for suggestions.
func WithSuggestionThreshold(threshold int) Option {
	return func(d *Decoder) {
		if threshold > 0 {
			d.suggestionThreshold = threshold
		}
	}
}

// Decoder performs strict YAML decoding with unknown-field diagnostics and suggestions.
type Decoder struct {
	pattern             *regexp.Regexp
	suggestionThreshold int
	contexts            map[string]FieldContext
}

// New constructs a Decoder configured with the given options.
func New(opts ...Option) *Decoder {
	d := &Decoder{
		pattern:             unknownFieldPattern,
		suggestionThreshold: DefaultSuggestionThreshold,
		contexts:            make(map[string]FieldContext),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Decode decodes data into target with unknown-field rejection enabled.
// If unknown fields or other decoding errors occur, it returns a *DecodeError.
func (d *Decoder) Decode(data []byte, target any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(target); err != nil {
		return d.wrapError(err)
	}
	return nil
}

// wrapError inspects err and, if it is a yaml.TypeError, rewrites it into a
// structured *DecodeError containing parsed unknown-field diagnostics and other errors.
func (d *Decoder) wrapError(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}

	decErr := &DecodeError{
		UnknownFields: make([]UnknownField, 0, len(typeErr.Errors)),
		OtherErrors:   make([]string, 0, len(typeErr.Errors)),
	}

	for _, raw := range typeErr.Errors {
		uf, ok := d.parseUnknownField(raw)
		if !ok {
			decErr.OtherErrors = append(decErr.OtherErrors, raw)
			continue
		}
		decErr.UnknownFields = append(decErr.UnknownFields, uf)
	}

	return decErr
}

// parseUnknownField attempts to parse one raw diagnostic string matching yaml.v3's
// "field not found" pattern into an UnknownField with a nearest-key suggestion.
func (d *Decoder) parseUnknownField(raw string) (UnknownField, bool) {
	match := d.pattern.FindStringSubmatch(raw)
	if match == nil {
		return UnknownField{}, false
	}
	lineNoStr, field, typeName := match[1], match[2], match[3]
	lineNo, _ := strconv.Atoi(lineNoStr)

	ctx, known := d.contexts[typeName]
	if !known {
		if dot := strings.LastIndex(typeName, "."); dot >= 0 {
			ctx, known = d.contexts[typeName[dot+1:]]
		}
	}

	label := "key"
	if known && ctx.Label != "" {
		label = ctx.Label
	}

	uf := UnknownField{
		Line:     lineNo,
		Field:    field,
		TypeName: typeName,
		Label:    label,
	}

	if known && ctx.Schema != nil {
		candidates := d.validKeys(ctx.Schema)
		if suggestion, found := str.Closest(
			field, candidates, d.suggestionThreshold,
		); found {
			uf.Suggestion = suggestion
		}
	}

	return uf, true
}

// validKeys inspects a struct type and returns all yaml key names declared on its
// fields, omitting unnamed, untagged, or "-" tagged fields.
func (d *Decoder) validKeys(schema reflect.Type) []string {
	for schema.Kind() == reflect.Pointer {
		schema = schema.Elem()
	}
	if schema.Kind() != reflect.Struct {
		return nil
	}

	keys := make([]string, 0, schema.NumField())
	for f := range schema.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}
