// Package schema versions the data the app stores, and upgrades older data.
//
// One number covers the whole data structure: the data folder records it in the
// schema-version file, and each station list carries it in its "version" field, so an
// exported list can be upgraded on its own.
//
// Every change to the stored structure, even a new optional field, must raise Current
// and add a Step. TestShape fails when the structure changes without this. See "Changing
// the stored data" in CONTRIBUTING.md.
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"denon-synology-vtuner/internal/atomicfile"
)

// Current is the data version this build reads and writes.
const Current = 2

// steps upgrades data to Current, in order. The first step has To: 2.
var steps = []Step{
	{To: 2, Catalog: genreToGenres},
}

// genreToGenres turns each virtual folder's "genre" into a "genres" list with that one
// genre, so that a filter can hold several.
func genreToGenres(catalog map[string]any) error {
	var walk func(folders any)
	walk = func(folders any) {
		list, _ := folders.([]any)
		for _, x := range list {
			folder, _ := x.(map[string]any)
			if filter, ok := folder["filter"].(map[string]any); ok {
				if g, ok := filter["genre"].(string); ok {
					delete(filter, "genre")
					if strings.TrimSpace(g) != "" {
						filter["genres"] = []any{g}
					}
				}
			}
			walk(folder["folders"])
		}
	}
	walk(catalog["folders"])
	return nil
}

// VersionFile is the file in the data folder that records the data version.
// The package's preupgrade script reads it too, so it holds only the number.
const VersionFile = "schema-version"

// upgradingFile exists during an upgrade and names the copy of the old data.
const upgradingFile = "schema-upgrading"

// keepBackups is how many copies from earlier upgrades stay in the backups folder.
const keepBackups = 3

// Step upgrades data from version To-1 to version To. Leave a function nil when the step
// has nothing to change there.
type Step struct {
	To int
	// Catalog changes one station list, decoded from JSON. It runs on the receivers'
	// lists and on imported files. The caller sets the version field.
	Catalog func(catalog map[string]any) error
	// Folder changes the other files in the data folder (devices.json, settings.env).
	// It must be safe to run again, because a power cut can interrupt an upgrade.
	Folder func(dir string) error
}

// Plan is a data version and the steps that lead to it.
type Plan struct {
	Current int
	Steps   []Step
}

// Default is the plan this build uses.
var Default = Plan{Current: Current, Steps: steps}

// NewerError reports data written by a newer version than this build understands.
type NewerError struct{ Found, Supported int }

func (e *NewerError) Error() string {
	return fmt.Sprintf("the data has version %d, but this version of the app only understands up to version %d: install a newer version of the app", e.Found, e.Supported)
}

// Check reports whether the plan's steps lead from version 1 to Current without a gap.
func (p Plan) Check() error {
	if p.Current != len(p.Steps)+1 {
		return fmt.Errorf("data version %d needs %d migration steps, found %d", p.Current, p.Current-1, len(p.Steps))
	}
	for i, s := range p.Steps {
		if s.To != i+2 {
			return fmt.Errorf("migration step %d must have To: %d, found %d", i+1, i+2, s.To)
		}
	}
	return nil
}

// UpgradeCatalog returns station list JSON at the current version. A list with no version
// is version 1. It refuses a list from a newer version.
func UpgradeCatalog(b []byte) ([]byte, error) { return Default.UpgradeCatalog(b) }

// UpgradeFolder brings the data folder to the current version. See Plan.UpgradeFolder.
func UpgradeFolder(dir string) error { return Default.UpgradeFolder(dir) }

// UpgradeCatalog returns station list JSON at the plan's version.
func (p Plan) UpgradeCatalog(b []byte) ([]byte, error) {
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	found := max(head.Version, 1)
	if head.Version < 0 {
		return nil, fmt.Errorf("invalid data version %d", head.Version)
	}
	if found > p.Current {
		return nil, &NewerError{found, p.Current}
	}
	if found == p.Current {
		return b, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber() // keeps large ids exact
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	for _, s := range p.Steps {
		if s.To <= found {
			continue
		}
		if s.Catalog != nil {
			if err := s.Catalog(doc); err != nil {
				return nil, fmt.Errorf("upgrading a station list to data version %d: %w", s.To, err)
			}
		}
		doc["version"] = s.To
	}
	return json.MarshalIndent(doc, "", "  ")
}

// UpgradeFolder brings the data folder to the plan's version, and is called before anything
// else reads it. A copy of the data goes to the backups folder before the first change. If
// a step fails, or the data is newer than the plan, it returns an error, and the app must
// not start. An interrupted upgrade continues on the next start.
func (p Plan) UpgradeFolder(dir string) error {
	if err := p.Check(); err != nil {
		return err
	}
	found, err := readVersion(dir)
	if err != nil {
		return err
	}
	if found > p.Current {
		return &NewerError{found, p.Current}
	}
	if found == 0 || found == p.Current {
		return recordVersion(dir, p.Current)
	}
	backup, err := startUpgrade(dir, found)
	if err != nil {
		return fmt.Errorf("copying the data before the upgrade: %w", err)
	}
	log.Printf("upgrading data from version %d to %d, copy of the old data in %s", found, p.Current, backup)
	for _, s := range p.Steps {
		if s.To <= found {
			continue
		}
		if err := p.runStep(dir, s); err != nil {
			return fmt.Errorf("upgrading data to version %d: %w (the old data is in %s)", s.To, err, backup)
		}
	}
	return finishUpgrade(dir)
}

func (p Plan) runStep(dir string, s Step) error {
	lists, err := filepath.Glob(filepath.Join(dir, "devices", "*.json"))
	if err != nil {
		return err
	}
	// Each step upgrades lists only as far as itself, so that a later step sees what it expects.
	one := Plan{Current: s.To, Steps: p.Steps[:s.To-1]}
	for _, path := range lists {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out, err := one.UpgradeCatalog(b)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := atomicfile.Write(path, out, 0o600); err != nil {
			return err
		}
	}
	if s.Folder != nil {
		if err := s.Folder(dir); err != nil {
			return err
		}
	}
	return writeVersion(dir, s.To)
}

// readVersion returns the folder's data version, or 0 for a folder with no data yet.
// Data saved before versions were recorded is version 1.
func readVersion(dir string) (int, error) {
	path := filepath.Join(dir, VersionFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(filepath.Join(dir, "devices.json")); errors.Is(err, os.ErrNotExist) {
			return 0, nil
		} else if err != nil {
			return 0, err
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || v < 1 {
		return 0, fmt.Errorf("%s doesn't hold a data version", path)
	}
	return v, nil
}

// recordVersion writes the version file unless it already holds v.
func recordVersion(dir string, v int) error {
	if b, err := os.ReadFile(filepath.Join(dir, VersionFile)); err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(v) {
		return nil
	}
	return writeVersion(dir, v)
}

func writeVersion(dir string, v int) error {
	return atomicfile.Write(filepath.Join(dir, VersionFile), []byte(strconv.Itoa(v)+"\n"), 0o644)
}

// startUpgrade copies the station lists, receivers and settings to the backups folder and
// returns the copy's path. The admin token and logs stay out of it. An upgrade that failed
// or was interrupted keeps its first copy, because later attempts start from changed data.
func startUpgrade(dir string, version int) (string, error) {
	marker := filepath.Join(dir, upgradingFile)
	if b, err := os.ReadFile(marker); err == nil {
		return strings.TrimSpace(string(b)), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	dest := filepath.Join(dir, "backups", fmt.Sprintf("%s-data-v%d", time.Now().UTC().Format("20060102-150405"), version))
	if err := os.MkdirAll(filepath.Join(dest, "devices"), 0o700); err != nil {
		return "", err
	}
	files, err := filepath.Glob(filepath.Join(dir, "devices", "*.json"))
	if err != nil {
		return "", err
	}
	for _, name := range []string{"devices.json", "settings.env", VersionFile} {
		files = append(files, filepath.Join(dir, name))
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return "", err
		}
		if err := atomicfile.Write(filepath.Join(dest, rel), b, 0o600); err != nil {
			return "", err
		}
	}
	// The marker comes last, so that it only ever names a complete copy.
	return dest, atomicfile.Write(marker, []byte(dest+"\n"), 0o600)
}

// finishUpgrade ends the upgrade and removes the oldest copies beyond keepBackups.
func finishUpgrade(dir string) error {
	root := filepath.Join(dir, "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for i := 0; i < len(entries)-keepBackups; i++ { // ReadDir sorts by name, which starts with the date
		if err := os.RemoveAll(filepath.Join(root, entries[i].Name())); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(dir, upgradingFile))
}
