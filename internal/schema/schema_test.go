package schema

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// twoSteps renames "nextId" to "next", then adds a field that needs the new name.
func twoSteps() Plan {
	return Plan{Current: 3, Steps: []Step{
		{To: 2, Catalog: func(c map[string]any) error { c["next"] = c["nextId"]; delete(c, "nextId"); return nil },
			Folder: func(dir string) error { return os.WriteFile(filepath.Join(dir, "step2"), nil, 0o600) }},
		{To: 3, Catalog: func(c map[string]any) error { c["copy"] = c["next"]; return nil }},
	}}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func legacyFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "devices.json"), `[]`)
	write(t, filepath.Join(dir, "settings.env"), "A=1\n")
	write(t, filepath.Join(dir, "admin.token"), "secret")
	write(t, filepath.Join(dir, "devices", "default.json"), `{"version":1,"nextId":1073741823}`)
	return dir
}

func TestDefaultPlanIsComplete(t *testing.T) {
	if err := Default.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFindsAMissingStep(t *testing.T) {
	if err := (Plan{Current: 2}).Check(); err == nil {
		t.Fatal("no error for a version with no step")
	}
	if err := (Plan{Current: 2, Steps: []Step{{To: 3}}}).Check(); err == nil {
		t.Fatal("no error for a step with the wrong number")
	}
}

func TestUpgradeCatalogRunsStepsInOrder(t *testing.T) {
	out, err := twoSteps().UpgradeCatalog([]byte(`{"nextId":1073741823}`))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["version"] != float64(3) || got["copy"] != float64(1073741823) || got["nextId"] != nil {
		t.Fatalf("got %s", out)
	}
	// The number must stay exact, not turn into 1.073741823e+09.
	if !strings.Contains(string(out), "1073741823") {
		t.Fatalf("id changed form: %s", out)
	}
}

func TestUpgradeCatalogLeavesCurrentAlone(t *testing.T) {
	in := `{"version":3, "x":1}`
	out, err := twoSteps().UpgradeCatalog([]byte(in))
	if err != nil || string(out) != in {
		t.Fatalf("got %s, %v", out, err)
	}
}

func TestUpgradeCatalogRefusesNewer(t *testing.T) {
	var newer *NewerError
	if _, err := twoSteps().UpgradeCatalog([]byte(`{"version":4}`)); !errors.As(err, &newer) {
		t.Fatalf("got %v", err)
	}
}

func TestUpgradeFolder(t *testing.T) {
	dir := legacyFolder(t)
	if err := twoSteps().UpgradeFolder(dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, VersionFile)); got != "3\n" {
		t.Fatalf("version file %q", got)
	}
	if got := read(t, filepath.Join(dir, "devices", "default.json")); !strings.Contains(got, `"copy": 1073741823`) {
		t.Fatalf("list not upgraded: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "step2")); err != nil {
		t.Fatal("folder step did not run")
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "*-data-v1"))
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	if got := read(t, filepath.Join(backups[0], "devices", "default.json")); !strings.Contains(got, `"nextId"`) {
		t.Fatalf("backup is not the old data: %s", got)
	}
	if read(t, filepath.Join(backups[0], "settings.env")) != "A=1\n" {
		t.Fatal("settings not in the backup")
	}
	if _, err := os.Stat(filepath.Join(backups[0], "admin.token")); err == nil {
		t.Fatal("the admin token must stay out of the backup")
	}
}

func TestFreshFolderGetsCurrentVersionAndNoBackup(t *testing.T) {
	dir := t.TempDir()
	if err := twoSteps().UpgradeFolder(dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, VersionFile)); got != "3\n" {
		t.Fatalf("version file %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "backups")); err == nil {
		t.Fatal("a new folder needs no backup")
	}
}

func TestNewerFolderIsRefusedAndUntouched(t *testing.T) {
	dir := legacyFolder(t)
	write(t, filepath.Join(dir, VersionFile), "9\n")
	before := read(t, filepath.Join(dir, "devices", "default.json"))
	var newer *NewerError
	if err := twoSteps().UpgradeFolder(dir); !errors.As(err, &newer) {
		t.Fatalf("got %v", err)
	}
	if read(t, filepath.Join(dir, VersionFile)) != "9\n" || read(t, filepath.Join(dir, "devices", "default.json")) != before {
		t.Fatal("data changed")
	}
}

func TestDamagedVersionFileIsRefused(t *testing.T) {
	dir := legacyFolder(t)
	write(t, filepath.Join(dir, VersionFile), "abc")
	if err := twoSteps().UpgradeFolder(dir); err == nil {
		t.Fatal("no error")
	}
}

func TestFailedStepKeepsVersionAndContinuesLater(t *testing.T) {
	dir := legacyFolder(t)
	p := twoSteps()
	fail := true
	p.Steps[1].Folder = func(string) error {
		if fail {
			return errors.New("disk trouble")
		}
		return nil
	}
	err := p.UpgradeFolder(dir)
	if err == nil || !strings.Contains(err.Error(), "backups") {
		t.Fatalf("the error must point to the old data: %v", err)
	}
	if got := read(t, filepath.Join(dir, VersionFile)); got != "2\n" {
		t.Fatalf("version after the failed step: %q", got)
	}
	// Retrying must not replace the copy of the old data with half-upgraded data.
	if err := p.UpgradeFolder(dir); err == nil {
		t.Fatal("no error on the second attempt")
	}
	fail = false
	if err := p.UpgradeFolder(dir); err != nil {
		t.Fatal(err)
	}
	backups, _ := os.ReadDir(filepath.Join(dir, "backups"))
	if len(backups) != 1 || !strings.HasSuffix(backups[0].Name(), "-data-v1") {
		t.Fatalf("backups: %v", backups)
	}
	if _, err := os.Stat(filepath.Join(dir, upgradingFile)); err == nil {
		t.Fatal("marker left after the upgrade")
	}
	if got := read(t, filepath.Join(dir, VersionFile)); got != "3\n" {
		t.Fatalf("version after the second run: %q", got)
	}
}

func TestDamagedListStopsTheUpgrade(t *testing.T) {
	dir := legacyFolder(t)
	write(t, filepath.Join(dir, "devices", "aabbccddeeff.json"), `{broken`)
	if err := twoSteps().UpgradeFolder(dir); err == nil || !strings.Contains(err.Error(), "aabbccddeeff.json") {
		t.Fatalf("the error must name the file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, VersionFile)); err == nil {
		t.Fatal("version recorded despite the failure")
	}
}

func TestOldBackupsAreRemoved(t *testing.T) {
	dir := legacyFolder(t)
	for _, n := range []string{"20200101-000000-data-v1", "20200102-000000-data-v1", "20200103-000000-data-v1"} {
		if err := os.MkdirAll(filepath.Join(dir, "backups", n), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := twoSteps().UpgradeFolder(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "backups"))
	if len(entries) != keepBackups || entries[0].Name() != "20200102-000000-data-v1" {
		t.Fatalf("backups: %v", entries)
	}
}

// One genre becomes a list of one, at any depth, and other filter fields stay as they were.
func TestUpgradeGenreToGenres(t *testing.T) {
	out, err := Default.UpgradeCatalog([]byte(`{"version":1,"nextId":1073741823,"folders":[
		{"id":1,"name":"Top","filter":{"country":"PL","genre":"rock"}},
		{"id":2,"name":"Parent","folders":[{"id":3,"name":"Inside","filter":{"genre":"80s"}}]},
		{"id":4,"name":"No genre","filter":{"country":"PL"}},
		{"id":5,"name":"Blank","filter":{"country":"PL","genre":" "}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version int
		NextID  int64 `json:"nextId"`
		Folders []struct {
			Filter  map[string]any
			Folders []struct{ Filter map[string]any }
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != Current || got.NextID != 1073741823 {
		t.Fatalf("%s", out)
	}
	genres := func(f map[string]any) any { return f["genres"] }
	if g, _ := genres(got.Folders[0].Filter).([]any); len(g) != 1 || g[0] != "rock" || got.Folders[0].Filter["country"] != "PL" {
		t.Fatalf("top level: %s", out)
	}
	if g, _ := genres(got.Folders[1].Folders[0].Filter).([]any); len(g) != 1 || g[0] != "80s" {
		t.Fatalf("inside a folder: %s", out)
	}
	if genres(got.Folders[2].Filter) != nil || genres(got.Folders[3].Filter) != nil {
		t.Fatalf("no genre to carry over: %s", out)
	}
	if strings.Contains(string(out), `"genre"`) {
		t.Fatalf("old field left: %s", out)
	}
}
