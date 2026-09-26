package cli

import "github.com/go-envx/envx/app/internal/shared/flags"

// forceFlag overwrites existing files instead of stopping on conflicts.
var forceFlag = flags.Spec[bool]{
	Name:  "force",
	Usage: "overwrite existing files",
}

// targetDirFlag is the directory to scaffold into.
var targetDirFlag = flags.Spec[string]{
	Name:  "target-dir",
	Usage: "directory to scaffold into",
}

// targetDirFor returns a targetDirFlag specification defaulted to templateName.
func targetDirFor(templateName string) *flags.Spec[string] {
	spec := targetDirFlag
	spec.Default = templateName
	return &spec
}
