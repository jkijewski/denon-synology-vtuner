package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"denon-synology-vtuner/internal/schema"
)

func TestParseRefusesBadInput(t *testing.T) {
	st := `{"name":"a","url":"http://stream.example.com/a"`
	for name, body := range map[string]string{
		"null station":    `{"stations":[null],"folders":[]}`,
		"null folder":     `{"stations":[],"folders":[null]}`,
		"null sub-folder": `{"stations":[],"folders":[{"name":"a","folders":[null]}]}`,
		"negative id":     `{"stations":[` + st + `,"id":-5}],"folders":[]}`,
		"id too high":     `{"stations":[` + st + fmt.Sprintf(`,"id":%d}],"folders":[]}`, MaxID+1),
		"folder id high":  fmt.Sprintf(`{"stations":[],"folders":[{"name":"a","id":%d}]}`, MaxID+1),
		"nextId too high": fmt.Sprintf(`{"nextId":%d,"stations":[],"folders":[]}`, MaxID+2),
		"negative nextId": `{"nextId":-1,"stations":[],"folders":[]}`,
		"negative rate":   `{"stations":[` + st + `,"bitrate":-1}],"folders":[]}`,
		"long name":       `{"stations":[{"name":"` + strings.Repeat("x", maxNameLen+1) + `","url":"http://stream.example.com/a"}],"folders":[]}`,
		"trailing data":   `{"stations":[],"folders":[]} trailing`,
		"second document": `{"stations":[],"folders":[]} {}`,
		"empty filter":    `{"stations":[],"folders":[{"name":"a","filter":{}}]}`,
		"blank folder":    `{"stations":[],"folders":[{"name":" \t "}]}`,
		"invisible name":  `{"stations":[],"folders":[{"name":"\u200b\u00a0"}]}`,
		"invisible st":    `{"stations":[{"name":"\u200b","url":"http://stream.example.com/a"}],"folders":[]}`,
		"bad order":       `{"stations":[],"folders":[{"name":"a","filter":{"order":"random"}}]}`,
		"negative rate f": `{"stations":[],"folders":[{"name":"a","filter":{"minBitrate":-1}}]}`,
		"negative limit":  `{"stations":[],"folders":[{"name":"a","filter":{"limit":-1}}]}`,
		"limit too high":  `{"stations":[],"folders":[{"name":"a","filter":{"limit":1001}}]}`,
		"long language":   `{"stations":[],"folders":[{"name":"a","filter":{"language":"` + strings.Repeat("x", maxShortLen+1) + `"}}]}`,
		"bad country":     `{"stations":[],"folders":[{"name":"a","filter":{"country":"Poland"}}]}`,
		"bad format":      `{"stations":[],"folders":[{"name":"a","filter":{"genres":["rock"],"format":"WMA"}}]}`,
		"long genre":      `{"stations":[],"folders":[{"name":"a","filter":{"genres":["` + strings.Repeat("x", maxShortLen+1) + `"]}}]}`,
		"too many genres": `{"stations":[],"folders":[{"name":"a","filter":{"genres":["a","b","c","d","e","f","g","h","i","j","k"]}}]}`,
		"blank genres":    `{"stations":[],"folders":[{"name":"a","filter":{"genres":[" ",""]}}]}`,
		"filter+folders":  `{"stations":[],"folders":[{"name":"a","filter":{"genres":["rock"]},"folders":[{"name":"b"}]}]}`,
		"filter+stations": `{"stations":[` + st + `,"id":2000}],"folders":[{"name":"a","filter":{"genres":["rock"]},"stations":[2000]}]}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestIDsRunOutWithAnError(t *testing.T) {
	st := `{"name":"a","url":"http://stream.example.com/a"`
	body := `{"stations":[` + st + fmt.Sprintf(`,"id":%d},`, MaxID) + st + `}],"folders":[]}`
	if _, err := Parse([]byte(body)); err == nil {
		t.Fatal("new entry got an id past the limit")
	}
}

func TestPolishNamesAtTheLimitAreFine(t *testing.T) {
	body := `{"stations":[{"name":"` + strings.Repeat("ż", maxNameLen) + `","url":"http://stream.example.com/a"}],"folders":[]}`
	if _, err := Parse([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentMergesAllKept(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	base := len(s.Snapshot().Stations)
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Merge(&Data{Stations: []*Station{{Name: fmt.Sprint("s", i), URL: "http://stream.example.com/s"}}})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := len(s.Snapshot().Stations) - base; got != 30 {
		t.Fatalf("kept %d of 30", got)
	}
}

func TestConcurrentReplaceKeepsDiskAndMemoryEqual(t *testing.T) {
	for range 30 {
		p := filepath.Join(t.TempDir(), "c.json")
		s, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := s.Replace(&Data{Stations: []*Station{{Name: fmt.Sprint("n", i), URL: "http://stream.example.com/s"}}, Folders: []*Folder{}}); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		if d.Stations[0].Name != s.Snapshot().Stations[0].Name {
			t.Fatal("disk and memory differ")
		}
	}
}

func TestReplaceIf(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, etag := s.SnapshotETag()
	if err := s.ReplaceIf(etag, d); err != nil {
		t.Fatal(err)
	}
	d2, etag2 := s.SnapshotETag()
	d2.Stations[0].Name = "changed"
	if err := s.ReplaceIf(etag2, d2); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceIf(etag2, d2); err != ErrConflict {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := s.ReplaceIf("", d2); err != nil {
		t.Fatal(err)
	}
}

// Lists saved by older versions must keep loading: 0.1.0-0003 knew no filters at all, and
// the first virtual folders knew only country, genre, format and name.
func TestOlderDataFilesLoad(t *testing.T) {
	for name, body := range map[string]string{
		"no filters": `{"version":1,"nextId":1002,"stations":[{"id":1000,"name":"Example FM","url":"http://stream.example.com/a"}],
			"folders":[{"id":1001,"name":"My Stations","stations":[1000]},{"id":1,"name":"Empty"}]}`,
		"first filters": `{"version":1,"nextId":1002,"stations":[],
			"folders":[{"id":1001,"name":"Polish rock","filter":{"country":"PL","genre":"rock","format":"MP3","name":"fm"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "c.json")
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			s, err := Open(p)
			if err != nil {
				t.Fatal(err)
			}
			if es, _, ok := s.List(0); !ok || len(es) == 0 {
				t.Fatalf("%+v", es)
			}
			// Saving it again writes the same lists back.
			if err := s.Replace(s.Snapshot()); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(p); err != nil {
				t.Fatal(err)
			}
		})
	}
	s, _ := Open(filepath.Join(t.TempDir(), "c.json"))
	d, err := Parse([]byte(`{"folders":[{"id":1001,"name":"Polish rock","filter":{"country":"PL","genre":"rock"}}],"stations":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(d); err != nil {
		t.Fatal(err)
	}
	if got := s.Filter(1001); got == nil || got.Country != "PL" || len(got.Genres) != 1 || got.Genres[0] != "rock" || got.Limit != 0 || got.Order != "" || got.MinBitrate != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestAnyOneFilterIsEnough(t *testing.T) {
	for _, f := range []string{`{"format":"MP3"}`, `{"language":"polish"}`, `{"minBitrate":128}`, `{"order":"votes"}`, `{"limit":20}`} {
		if _, err := Parse([]byte(`{"stations":[],"folders":[{"name":"a","filter":` + f + `}]}`)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestInvisibleCharactersLeaveNames(t *testing.T) {
	d, err := Parse([]byte(`{"stations":[{"name":"\u200b Example \u00a0","url":"http://stream.example.com/a"}],"folders":[{"name":"\ufeffJazz\n"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Stations[0].Name != "Example" || d.Folders[0].Name != "Jazz" {
		t.Fatalf("%q %q", d.Stations[0].Name, d.Folders[0].Name)
	}
}

func TestFilterIsCleanedAndKept(t *testing.T) {
	d, err := Parse([]byte(`{"stations":[],"folders":[{"name":"a","filter":{"country":" pl ","genres":[" rock ","80s","ROCK",""],"format":"mp3","name":" fm ","language":" Polish ","minBitrate":128,"order":" Votes ","limit":25}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := d.Folders[0].Filter
	if f.Country != "PL" || strings.Join(f.Genres, ",") != "rock,80s" || f.Format != "MP3" || f.Name != "fm" || f.Language != "polish" || f.MinBitrate != 128 || f.Order != "votes" || f.Limit != 25 {
		t.Fatalf("%+v", f)
	}
	s, err := Open(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Replace(d); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	id := d.Folders[0].ID
	if got := s2.Filter(id); got == nil || !reflect.DeepEqual(*got, *f) {
		t.Fatalf("filter after reload: %+v", got)
	}
	if err := s2.Merge(s2.Snapshot()); err != nil {
		t.Fatal(err)
	}
	if m := s2.Snapshot().Folders; len(m) != 2 || m[1].Filter == nil || !reflect.DeepEqual(*m[1].Filter, *f) {
		t.Fatalf("filter after merge: %+v", m)
	}
}

func TestParseRefusesListFromNewerVersion(t *testing.T) {
	_, err := Parse([]byte(`{"version":999,"stations":[],"folders":[]}`))
	var newer *schema.NewerError
	if !errors.As(err, &newer) {
		t.Fatalf("got %v", err)
	}
}

func TestParseTreatsMissingVersionAsCurrentFormat(t *testing.T) {
	d, err := Parse([]byte(`{"stations":[],"folders":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != schema.Current {
		t.Fatalf("version %d", d.Version)
	}
}
