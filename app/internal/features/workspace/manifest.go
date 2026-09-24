package workspace

// ManifestDoc is a parsed, validated manifest together with the location it
// was read from and the block indentation detected in the source document.
type ManifestDoc struct {
	// Content is the parsed, validated manifest content.
	Content *Manifest
	// Path is the absolute path the manifest was read from.
	Path string
	// Indent is the detected block indentation width, defaulting to two spaces
	// when the source document has none to detect.
	Indent int
}

// DefaultEnvironment returns the first declared environment, or an empty string
// if no environments are declared.
func (m *ManifestDoc) DefaultEnvironment() string {
	if m == nil || m.Content == nil {
		return ""
	}
	return m.Content.DefaultEnvironment()
}

// HasEnvironment reports whether env is declared in the manifest environments list.
func (m *ManifestDoc) HasEnvironment(env string) bool {
	if m == nil || m.Content == nil {
		return false
	}
	return m.Content.HasEnvironment(env)
}

// HasInclude reports whether any project declares includePath in its include list.
func (m *ManifestDoc) HasInclude(includePath string) bool {
	if m == nil || m.Content == nil {
		return false
	}
	return m.Content.HasInclude(includePath)
}

// LookupProject finds a project by name, returning its definition and if one was found.
func (m *ManifestDoc) LookupProject(name string) (Project, bool) {
	if m == nil || m.Content == nil {
		return Project{}, false
	}
	return m.Content.LookupProject(name)
}
