package devices

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"denon-synology-vtuner/internal/catalog"
)

func TestSeparateListsPerReceiver(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Login("00:11:22:33:44:55", "192.168.1.50")
	b, _ := r.Login("001122334466", "192.168.1.51")
	if a != "001122334455" || b != "001122334466" {
		t.Fatal(a, b)
	}
	sa, _ := r.Store(a)
	sb, _ := r.Store(b)
	d := sa.Snapshot()
	d.Stations = append(d.Stations, &catalog.Station{Name: "Only A", URL: "http://stream.example.com/a"})
	if err := sa.Replace(d); err != nil {
		t.Fatal(err)
	}
	if len(sb.Snapshot().Stations) == len(sa.Snapshot().Stations) {
		t.Fatal("lists are shared")
	}
	if r.ForIP("192.168.1.51") != b || r.ForIP("192.168.1.99") != DefaultKey {
		t.Fatal("ip mapping")
	}
	// reopen keeps receivers
	r2, _ := Open(dir)
	if len(r2.List()) != 3 || r2.List()[0].Key != DefaultKey {
		t.Fatalf("%+v", r2.List())
	}
	s2, _ := r2.Store(a)
	if len(s2.Snapshot().Stations) != len(sa.Snapshot().Stations) {
		t.Fatal("not persisted")
	}
}

func TestBadMACUsesDefault(t *testing.T) {
	r, _ := Open(t.TempDir())
	k, _ := r.Login("not-a-mac", "192.168.1.50")
	if k != DefaultKey {
		t.Fatal(k)
	}
	if r.Delete(DefaultKey) == nil {
		t.Fatal("default must not be deletable")
	}
}

func TestOpenRecoversFromDamagedRegistry(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "garbage": "{not json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			r, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			key, err := r.Login("00:11:22:33:44:55", "192.168.1.50")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "devices.json"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			r, err = Open(dir)
			if err != nil {
				t.Fatalf("service would not start: %v", err)
			}
			if _, err := r.Store(key); err != nil {
				t.Fatalf("receiver lost: %v", err)
			}
			if _, err := r.Store(DefaultKey); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, "devices.json.damaged")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReceiverLimit(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range MaxDevices + 20 {
		if _, err := r.Login(fmt.Sprintf("%012x", i+1), "192.168.1.50"); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(r.List()); got != MaxDevices {
		t.Fatalf("%d receivers", got)
	}
	files, _ := os.ReadDir(filepath.Join(dir, "devices"))
	if len(files) > MaxDevices {
		t.Fatalf("%d list files", len(files))
	}
	key, err := r.Login("ffffffffffff", "192.168.1.99")
	if err != nil || key != DefaultKey {
		t.Fatalf("%q %v", key, err)
	}
	// Receivers already known keep working at the limit.
	if key, err := r.Login("000000000001", "192.168.1.51"); err != nil || key != "000000000001" {
		t.Fatalf("%q %v", key, err)
	}
}

func TestRepeatLoginDoesNotRewriteRegistry(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Login("001122334455", "192.168.1.50"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "devices.json")
	if err := os.WriteFile(p, []byte("marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Login("001122334455", "192.168.1.50"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "marker" {
		t.Fatal("unchanged login wrote to disk")
	}
	if _, err := r.Login("001122334455", "192.168.1.51"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) == "marker" {
		t.Fatal("new address was not saved")
	}
}

func TestAddressTakenOverByAnotherReceiver(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r.Login("00000000000a", "192.168.1.50")
	r.Login("00000000000b", "192.168.1.50") // b now has a's old address
	r.Login("00000000000a", "192.168.1.51") // a moves on
	if got := r.ForIP("192.168.1.50"); got != "00000000000b" {
		t.Fatalf("%q", got)
	}
}
