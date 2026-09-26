package filex

import (
	"io/fs"
	"sort"
)

// CollectFiles returns every file path under src, relative to src and slash-separated,
// in sorted order for deterministic output.
func CollectFiles(src fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
