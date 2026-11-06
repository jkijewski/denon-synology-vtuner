package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testData() *Data {
	return &Data{
		Stations: []*Station{
			{ID: 1000, Name: "Example Jazz FM", URL: "http://stream.example.com/jazz.mp3"},
			{ID: 1001, Name: "Example Smooth", URL: "https://stream.example.com/smooth.aac", Description: "Łagodny jazz"},
		},
		Folders: []*Folder{
			{Name: "Jazz", Stations: []int{1000, 1001}},
			{Name: "Favourites", Stations: []int{1000}},
		},
	}
}

func TestStarterOnFirstOpen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "catalog.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if es, _, _ := s.List(0); len(es) == 0 {
		t.Fatal("starter catalog has no folders")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("starter catalog not saved")
	}
}

func TestIDsStableAndNotReused(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "c.json"))
	d := testData()
	d.Folders[0].Stations = nil
	d.Folders[1].Stations = nil
	if err := s.Replace(d); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	id := snap.Stations[0].ID
	snap.Stations[0].Name = "Renamed"
	if err := s.Replace(snap); err != nil {
		t.Fatal(err)
	}
	if st, ok := s.Station(id); !ok || st.Name != "Renamed" {
		t.Fatalf("edit changed id: %+v", st)
	}
	// delete the last station, then add a new one: the deleted id must not come back
	snap = s.Snapshot()
	last := snap.Stations[len(snap.Stations)-1].ID
	snap.Stations = snap.Stations[:len(snap.Stations)-1]
	_ = s.Replace(snap)
	snap = s.Snapshot()
	snap.Stations = append(snap.Stations, &Station{Name: "New", URL: "http://stream.example.com/new"})
	_ = s.Replace(snap)
	snap = s.Snapshot()
	if got := snap.Stations[len(snap.Stations)-1].ID; got == last {
		t.Fatalf("id %d was reused", got)
	}
}

func TestValidation(t *testing.T) {
	d := testData()
	d.Stations[0].URL = "ftp://example.com/stream"
	err := d.Validate()
	fe, ok := err.(*FieldError)
	if !ok || fe.Field != "url" {
		t.Fatalf("want url error, got %v", err)
	}
	d = testData()
	d.Stations[1].Name = "  "
	if fe, ok := d.Validate().(*FieldError); !ok || fe.Field != "name" {
		t.Fatalf("want name error")
	}
}

func TestStationInTwoFolders(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "c.json"))
	if err := s.Replace(testData()); err != nil {
		t.Fatal(err)
	}
	top, _, _ := s.List(0)
	a, _, _ := s.List(top[0].Folder.ID)
	b, _, _ := s.List(top[1].Folder.ID)
	if a[0].Station.ID != b[0].Station.ID {
		t.Fatal("same station should have same id in both folders")
	}
}

func TestExportImportRoundTripAndReload(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	s, _ := Open(p)
	_ = s.Replace(testData())
	want := s.Snapshot()
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, s2.Snapshot()) {
		t.Fatal("reload differs")
	}
}

func TestMergeReassignsIDs(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "c.json"))
	_ = s.Replace(testData())
	if err := s.Merge(testData()); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if len(d.Stations) != 4 || len(d.Folders) != 4 {
		t.Fatalf("merge: %d stations %d folders", len(d.Stations), len(d.Folders))
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchIgnoresCaseAndDiacritics(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "c.json"))
	_ = s.Replace(testData())
	if got := s.Search("JAZZ"); len(got) != 2 {
		t.Fatalf("want 2 hits, got %d", len(got))
	}
	if got := s.Search("lagodny"); len(got) != 1 {
		t.Fatalf("diacritics: want 1, got %d", len(got))
	}
	if got := s.Search("nothing-matches"); len(got) != 0 {
		t.Fatal("want no hits")
	}
}
