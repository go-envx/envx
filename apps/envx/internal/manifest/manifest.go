package manifest

import (
	"path/filepath"

	"github.com/go-envx/envx/apps/envx/internal/schema"
)

// -------------------------------------------------------------------------------------
// Loaded pairs a parsed schema.Manifest with the on-disk directory it was loaded
// from (the workspace root). The embedded *schema.Manifest promotes the pure
// schema queries (LookupProject, HasEnvironment, HasInclude); Loaded adds the
// location-aware operations that need the workspace root.
type Loaded struct {
	*schema.Manifest
	Dir string
}

// -------------------------------------------------------------------------------------
// LookupInclude finds an include path across all projects and resolves it to an
// absolute directory and base name against the workspace root. It returns false
// when no project declares the include. Used by the set action to locate a target
// overlay.
func (l *Loaded) LookupInclude(includePath string) (dir, name string, ok bool) {
	if !l.HasInclude(includePath) {
		return "", "", false
	}
	dir = filepath.Join(l.Dir, filepath.Dir(includePath))
	return dir, filepath.Base(includePath), true
}
