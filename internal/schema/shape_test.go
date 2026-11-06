package schema_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"denon-synology-vtuner/internal/catalog"
	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/schema"
)

var addShape = flag.Bool("add-shape", false, "write the shape file for a new data version; an existing one is never overwritten")

// shape lists every stored field with its JSON name and type.
func shape() string {
	seen := map[reflect.Type]bool{}
	var lines []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			walk(t.Elem())
		case reflect.Struct:
			if seen[t] || t.PkgPath() == "time" {
				return
			}
			seen[t] = true
			for i := range t.NumField() {
				f := t.Field(i)
				if tag := f.Tag.Get("json"); f.IsExported() && tag != "-" {
					lines = append(lines, fmt.Sprintf("%s.%s %s", t.Name(), tag, f.Type))
					walk(f.Type)
				}
			}
		}
	}
	walk(reflect.TypeOf(catalog.Data{}))
	walk(reflect.TypeOf(devices.Device{}))
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// TestShape fails when the stored structure changes without a new data version.
func TestShape(t *testing.T) {
	path := filepath.Join("testdata", fmt.Sprintf("shape-v%d.txt", schema.Current))
	want, err := os.ReadFile(path)
	if os.IsNotExist(err) && *addShape {
		if err := os.WriteFile(path, []byte(shape()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("%v\nFor a new data version, run: go test ./internal/schema -run TestShape -add-shape", err)
	}
	if got := shape(); got != string(want) {
		t.Fatalf(`the stored data structure changed without a new data version.
Raise schema.Current, add a migration step and a testdata/v<old version> sample, then run:
  go test ./internal/schema -run TestShape -add-shape
See "Changing the stored data" in CONTRIBUTING.md.

Saved for version %d:
%s
Now:
%s`, schema.Current, want, got)
	}
}

// TestSamplesFromEveryVersionLoad upgrades a saved data folder of every earlier version.
func TestSamplesFromEveryVersionLoad(t *testing.T) {
	for v := 1; v <= schema.Current; v++ {
		src := filepath.Join("testdata", fmt.Sprintf("v%d", v))
		t.Run(fmt.Sprintf("v%d", v), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
				t.Fatalf("every data version needs a sample in %s: %v", src, err)
			}
			if err := schema.UpgradeFolder(dir); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, schema.VersionFile))
			if err != nil || strings.TrimSpace(string(got)) != fmt.Sprint(schema.Current) {
				t.Fatalf("version file %q, %v", got, err)
			}
			reg, err := devices.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if n := len(reg.List()); n != 2 {
				t.Fatalf("%d receivers, want 2", n)
			}
			for _, key := range []string{devices.DefaultKey, "001122334455"} {
				store, err := reg.Store(key)
				if err != nil {
					t.Fatal(err)
				}
				d := store.Snapshot()
				if d.Version != schema.Current || len(d.Stations) != 1 || len(d.Folders) != 2 {
					t.Fatalf("list %s: version %d, %d stations, %d folders", key, d.Version, len(d.Stations), len(d.Folders))
				}
			}
		})
	}
}
