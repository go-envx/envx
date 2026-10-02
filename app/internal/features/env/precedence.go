package env

// PrecedenceString resolves a string setting: the explicit value wins when
// present, then the first non-empty layer (e.g. project then global default),
// and finally "". Nil and empty layers are skipped.
func PrecedenceString(explicit *string, layers ...*string) string {
	if explicit != nil {
		return *explicit
	}
	for _, layer := range layers {
		if layer != nil && *layer != "" {
			return *layer
		}
	}
	return ""
}

// PrecedenceBool resolves a boolean setting: the explicit value wins when
// present, then the first non-nil layer, and finally false.
func PrecedenceBool(explicit *bool, layers ...*bool) bool {
	if explicit != nil {
		return *explicit
	}
	for _, layer := range layers {
		if layer != nil {
			return *layer
		}
	}
	return false
}
