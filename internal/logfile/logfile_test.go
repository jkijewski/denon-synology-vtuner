package logfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateKeepsN(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	w := &Writer{Path: p, MaxBytes: 10, Keep: 2}
	for i := 0; i < 10; i++ {
		w.Write([]byte("123456\n"))
	}
	for _, s := range []string{"", ".1", ".2"} {
		if _, err := os.Stat(p + s); err != nil {
			t.Fatalf("missing %s", p+s)
		}
	}
	if _, err := os.Stat(p + ".3"); err == nil {
		t.Fatal(".3 should not exist")
	}
}
