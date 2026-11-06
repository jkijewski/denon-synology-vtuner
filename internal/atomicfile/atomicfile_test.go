package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.json")
	for _, want := range []string{"one", "two"} {
		if err := Write(p, []byte(want), 0o600); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(p); string(b) != want {
			t.Fatalf("%q", b)
		}
	}
	st, err := os.Stat(p)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("%v %v", st, err)
	}
	if e, _ := os.ReadDir(dir); len(e) != 1 {
		t.Fatalf("temp file left behind: %v", e)
	}
}
