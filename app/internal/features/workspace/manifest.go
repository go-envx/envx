package workspace

import "github.com/go-envx/envx/app/internal/schema"

// Manifest is a parsed, validated manifest together with the location it
// was read from and the block indentation detected in the source document.
type Manifest struct {
	// Content is the parsed, validated manifest content.
	Content *schema.Manifest
	// Path is the absolute path the manifest was read from.
	Path string
	// Indent is the detected block indentation width, defaulting to two spaces
	// when the source document has none to detect.
	Indent int
}

// DefaultEnvironment returns the first declared environment, or an empty string
// if no environments are declared.
func (m *Manifest) DefaultEnvironment() string {
	if m == nil || m.Content == nil {
		return ""
	}
	return m.Content.DefaultEnvironment()
}

// HasEnvironment reports whether env is declared in the manifest environments list.
func (m *Manifest) HasEnvironment(env string) bool {
	if m == nil || m.Content == nil {
		return false
	}
	return m.Content.HasEnvironment(env)
}

// HasInclude reports whether any project declares includePath in its include list.
func (m *Manifest) HasInclude(includePath string) bool {
	if m == nil || m.Content == nil {
		return false
	}
	return m.Content.HasInclude(includePath)
}

// LookupProject finds a project by name, returning its definition and if one was found.
func (m *Manifest) LookupProject(name string) (schema.Project, bool) {
	if m == nil || m.Content == nil {
		return schema.Project{}, false
	}
	return m.Content.LookupProject(name)
}
