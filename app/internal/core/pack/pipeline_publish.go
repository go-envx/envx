package pack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-envx/envx/app/internal/lib/atomicfile"
)

// scratchName names the fixed staging directory inside an existing
// destination, or suffixes the destination's name beside a new one. A
// leftover scratch directory holds what an interrupted pack could not restore.
const scratchName = ".envx-pack"

// bundle is the complete content of a pack before publication.
type bundle struct {
	// manifest is the rewritten manifest.
	manifest []byte
	// items are the namespace files with their content.
	items []copyItem
	// store is the exported secrets store, or nil when none is bundled.
	store []byte
}

// scratch is the staging area of one publication.
type scratch struct {
	// dir is the scratch directory.
	dir string
	// staged holds the bundle being written.
	staged string
	// previous holds replaced output until publication succeeds.
	previous string
}

// stage writes the bundle into scratch space and publishes it at outDir. An
// existing directory keeps its identity, mode, and mount and has its entries
// swapped; any other destination is swapped as a whole path, so a symlink is
// replaced rather than followed. Scratch space is removed unless it holds
// unrestored output.
func stage(outDir string, b bundle, force bool) error {
	inPlace := false
	if info, err := os.Stat(outDir); err == nil && info.IsDir() {
		link, err := os.Lstat(outDir)
		inPlace = err == nil && (link.IsDir() || !force)
	}
	dir := filepath.Join(outDir, scratchName)
	if !inPlace {
		parent := filepath.Dir(outDir)
		if err := os.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("creating output parent %s: %w", parent, err)
		}
		dir = filepath.Join(parent, "."+filepath.Base(outDir)+scratchName)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf(
				"%s is left from an interrupted pack; recover or remove it", dir,
			)
		}
		return fmt.Errorf("creating staging directory %s: %w", dir, err)
	}
	sc := scratch{
		dir:      dir,
		staged:   filepath.Join(dir, "bundle"),
		previous: filepath.Join(dir, "previous"),
	}
	published := false
	defer func() {
		_ = os.RemoveAll(sc.staged)
		if published {
			_ = os.RemoveAll(sc.previous)
		}
		// Remove refuses non-empty directories, keeping unrestored output.
		_ = os.Remove(sc.previous)
		_ = os.Remove(sc.dir)
	}()

	if err := sc.write(b); err != nil {
		return err
	}
	var err error
	if inPlace {
		err = sc.swapContents(outDir, force)
	} else {
		err = sc.swapPath(outDir, force)
	}
	published = err == nil
	return err
}

// files returns the bundle-relative paths of every bundle file, sorted.
func (b bundle) files() []string {
	files := []string{bundleManifestName}
	for _, item := range b.items {
		files = append(files, item.dest)
	}
	if b.store != nil {
		files = append(files, bundleSecretsName)
	}
	slices.Sort(files)
	return files
}

// write durably writes every bundle file under the staged directory.
func (sc scratch) write(b bundle) error {
	if err := os.Mkdir(sc.staged, 0o750); err != nil {
		return fmt.Errorf("creating staging directory %s: %w", sc.staged, err)
	}
	write := func(rel string, data []byte, writeFile func(string, []byte) error) error {
		path := filepath.Join(sc.staged, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("creating bundle directory for %s: %w", rel, err)
		}
		if err := writeFile(path, data); err != nil {
			return fmt.Errorf("writing bundle file %s: %w", rel, err)
		}
		return nil
	}
	if err := write(bundleManifestName, b.manifest, atomicfile.Write); err != nil {
		return err
	}
	for _, item := range b.items {
		if err := write(item.dest, item.data, atomicfile.Write); err != nil {
			return err
		}
	}
	if b.store != nil {
		return write(bundleSecretsName, b.store, atomicfile.WritePrivate)
	}
	return nil
}

// swapPath renames the staged bundle to target. A forced replacement first
// moves target aside and moves it back if the rename fails.
func (sc scratch) swapPath(target string, force bool) error {
	if force {
		err := os.Rename(target, sc.previous)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("moving aside output %s: %w", target, err)
		}
	}
	if err := os.Rename(sc.staged, target); err != nil {
		restoreErr := os.Rename(sc.previous, target)
		if restoreErr != nil && !errors.Is(restoreErr, fs.ErrNotExist) {
			return fmt.Errorf(
				"publishing bundle %s: %w (previous output remains at %s)",
				target, errors.Join(err, restoreErr), sc.previous,
			)
		}
		return fmt.Errorf("publishing bundle %s: %w", target, err)
	}
	return nil
}

// swapContents moves the staged entries into dir. A forced replacement first
// moves dir's entries aside. Any failure reverses every completed move.
func (sc scratch) swapContents(dir string, force bool) error {
	type move struct {
		// from is the entry's original path.
		from string
		// to is the entry's new path.
		to string
	}
	var moves []move
	rename := func(from, to string) error {
		if err := os.Rename(from, to); err != nil {
			return err
		}
		moves = append(moves, move{from: from, to: to})
		return nil
	}
	rollback := func(cause error) error {
		errs := []error{cause}
		for i := len(moves) - 1; i >= 0; i-- {
			if err := os.Rename(moves[i].to, moves[i].from); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 1 {
			return fmt.Errorf(
				"publishing bundle %s: %w (previous output remains under %s)",
				dir, errors.Join(errs...), sc.previous,
			)
		}
		return fmt.Errorf("publishing bundle %s: %w", dir, cause)
	}

	if force {
		if err := os.Mkdir(sc.previous, 0o700); err != nil {
			return fmt.Errorf("creating backup directory %s: %w", sc.previous, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("reading output directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.Name() == scratchName {
				continue
			}
			err := rename(
				filepath.Join(dir, entry.Name()), filepath.Join(sc.previous, entry.Name()),
			)
			if err != nil {
				return rollback(err)
			}
		}
	}
	entries, err := os.ReadDir(sc.staged)
	if err != nil {
		return rollback(err)
	}
	for _, entry := range entries {
		to := filepath.Join(dir, entry.Name())
		// Rename would silently replace a file that appeared since the check.
		if _, err := os.Lstat(to); err == nil {
			return rollback(fmt.Errorf("%w: %s", ErrOutputNotEmpty, dir))
		}
		if err := rename(filepath.Join(sc.staged, entry.Name()), to); err != nil {
			return rollback(err)
		}
	}
	return nil
}

// ensureOutDirEmpty rejects an existing non-directory or non-empty directory
// without modifying it. A missing path is accepted.
func ensureOutDirEmpty(outDir string) error {
	info, err := os.Stat(outDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspecting output directory %s: %w", outDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf(
			"%w: %s (use --force to replace it)", ErrOutputNotDirectory, outDir,
		)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("reading output directory %s: %w", outDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%w: %s (use --force to replace it)", ErrOutputNotEmpty, outDir)
	}
	return nil
}
