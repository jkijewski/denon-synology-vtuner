// Package catalog stores the user's folders and stations.
package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"denon-synology-vtuner/internal/atomicfile"
	"denon-synology-vtuner/internal/schema"
)

// Station is one radio station.
type Station struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Format      string `json:"format,omitempty"`   // e.g. MP3, AAC
	Bitrate     int    `json:"bitrate,omitempty"`  // kbit/s
	Location    string `json:"location,omitempty"` // country or city
	Logo        string `json:"logo,omitempty"`
	Relay       bool   `json:"relay,omitempty"`
	// Source is "radio-browser" for stations added from the public directory; SourceID is its station uuid.
	Source   string `json:"source,omitempty"`
	SourceID string `json:"sourceId,omitempty"`
}

// Folder holds sub-folders and station ids, both in user order. A folder with a Filter is
// virtual: it holds neither, and its stations come from the public directory when it is opened.
type Folder struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Folders  []*Folder `json:"folders,omitempty"`
	Stations []int     `json:"stations,omitempty"`
	Filter   *Filter   `json:"filter,omitempty"`
}

// Filter selects public directory stations for a virtual folder. All set fields must match.
// Any change here needs a new data version and a migration: see internal/schema.
type Filter struct {
	Country    string   `json:"country,omitempty"`    // ISO 3166-1 code, e.g. PL
	Genres     []string `json:"genres,omitempty"`     // a station needs any one of them
	Format     string   `json:"format,omitempty"`     // one of filterFormats
	Name       string   `json:"name,omitempty"`       // text in the station name
	Language   string   `json:"language,omitempty"`   // language name, e.g. polish
	MinBitrate int      `json:"minBitrate,omitempty"` // kbit/s
	Order      string   `json:"order,omitempty"`      // one of filterOrders; empty means most played first
	Limit      int      `json:"limit,omitempty"`      // most stations to list; 0 means the listing's own limit
}

// filterFormats are the formats the receiver can play, as offered by the directory.
var filterFormats = []string{"MP3", "AAC", "AAC+"}

// filterOrders are the sort orders offered for a virtual folder, as named by the directory.
var filterOrders = []string{"clickcount", "votes", "clicktrend", "bitrate", "name"}

const maxFilterLimit = 1000

// maxFilterGenres is the most genres a filter can hold. Each one is a directory request.
const maxFilterGenres = 10

// CleanName removes spaces and invisible characters from both ends of a name.
func CleanName(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || !unicode.IsGraphic(r) })
}

// Blank reports whether a name would show as nothing on a display.
func Blank(s string) bool { return CleanName(s) == "" }

// Normalize cleans the filter's fields and checks them. folder names the folder in error messages.
func (f *Filter) Normalize(folder string) error {
	entry := fmt.Sprintf("folder %q", folder)
	f.Country = strings.ToUpper(strings.TrimSpace(f.Country))
	f.Genres = cleanGenres(f.Genres)
	f.Format = strings.ToUpper(strings.TrimSpace(f.Format))
	f.Name = strings.TrimSpace(f.Name)
	f.Language = strings.ToLower(strings.TrimSpace(f.Language))
	f.Order = strings.ToLower(strings.TrimSpace(f.Order))
	if f.Country == "" && len(f.Genres) == 0 && f.Format == "" && f.Name == "" && f.Language == "" &&
		f.MinBitrate == 0 && f.Order == "" && f.Limit == 0 {
		return &FieldError{entry, "filter", "choose at least one filter"}
	}
	if utf8.RuneCountInString(f.Language) > maxShortLen {
		return &FieldError{entry, "language", fmt.Sprintf("longer than %d characters", maxShortLen)}
	}
	if f.MinBitrate < 0 || f.MinBitrate > maxBitrate {
		return &FieldError{entry, "minBitrate", fmt.Sprintf("must be from 0 to %d", maxBitrate)}
	}
	if f.Order != "" && !slices.Contains(filterOrders, f.Order) {
		return &FieldError{entry, "order", "must be " + strings.Join(filterOrders, ", ")}
	}
	if f.Limit < 0 || f.Limit > maxFilterLimit {
		return &FieldError{entry, "limit", fmt.Sprintf("must be from 0 to %d", maxFilterLimit)}
	}
	if f.Country != "" && !isCountryCode(f.Country) {
		return &FieldError{entry, "country", "must be a two-letter country code"}
	}
	if f.Format != "" && !slices.Contains(filterFormats, f.Format) {
		return &FieldError{entry, "format", "must be " + strings.Join(filterFormats, ", ")}
	}
	if len(f.Genres) > maxFilterGenres {
		return &FieldError{entry, "genres", fmt.Sprintf("choose at most %d", maxFilterGenres)}
	}
	for _, g := range f.Genres {
		if utf8.RuneCountInString(g) > maxShortLen {
			return &FieldError{entry, "genres", fmt.Sprintf("longer than %d characters", maxShortLen)}
		}
	}
	if utf8.RuneCountInString(f.Name) > maxShortLen {
		return &FieldError{entry, "name", fmt.Sprintf("longer than %d characters", maxShortLen)}
	}
	return nil
}

// cleanGenres trims the genres and drops blanks and repeats, ignoring case, keeping the order.
func cleanGenres(in []string) []string {
	var out []string
	for _, g := range in {
		g = strings.TrimSpace(g)
		if g != "" && !slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, g) }) {
			out = append(out, g)
		}
	}
	return out
}

func isCountryCode(s string) bool {
	return len(s) == 2 && s[0] >= 'A' && s[0] <= 'Z' && s[1] >= 'A' && s[1] <= 'Z'
}

// Data is the whole catalog as stored on disk and exchanged on import/export.
type Data struct {
	Version  int        `json:"version"`
	NextID   int        `json:"nextId"`
	Stations []*Station `json:"stations"`
	Folders  []*Folder  `json:"folders"`
}

// FieldError reports an invalid entry.
type FieldError struct {
	Entry string `json:"entry"`
	Field string `json:"field"`
	Msg   string `json:"message"`
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s: %s", e.Entry, e.Field, e.Msg) }

const (
	firstID = 1000
	// MaxID is the highest id a catalog entry can have. Higher numbers are the ids
	// of public directory stations, and also fit a 32-bit int on ARM packages.
	MaxID = 1<<30 - 1

	maxNameLen  = 500
	maxTextLen  = 2000
	maxURLLen   = 4096
	maxShortLen = 300
	maxBitrate  = 1000000
)

// Validate checks every station and folder and assigns ids to new entries (id 0).
// Ids are never reused: NextID only grows.
func (d *Data) Validate() error {
	if err := d.checkHeader(); err != nil {
		return err
	}
	if err := checkFolderEntries(d.Folders); err != nil {
		return err
	}
	d.raiseNextID()
	seen := map[int]bool{}
	if err := d.validateStations(seen); err != nil {
		return err
	}
	if err := d.validateFolders(seen); err != nil {
		return err
	}
	return d.checkFolderStations()
}

// checkHeader checks the version, NextID and station entries, so that the other steps can
// rely on them.
func (d *Data) checkHeader() error {
	if d.Version == 0 {
		d.Version = schema.Current
	}
	if d.Version != schema.Current {
		return &FieldError{"catalog", "version", fmt.Sprintf("must be %d", schema.Current)}
	}
	if d.NextID < 0 || d.NextID > MaxID+1 {
		return &FieldError{"catalog", "nextId", fmt.Sprintf("must be from 0 to %d", MaxID+1)}
	}
	for i, s := range d.Stations {
		if s == nil {
			return &FieldError{fmt.Sprintf("station #%d", i+1), "", "empty entry"}
		}
		if s.ID < 0 || s.ID > MaxID {
			return &FieldError{fmt.Sprintf("station #%d", i+1), "id", fmt.Sprintf("must be from 0 to %d", MaxID)}
		}
	}
	return nil
}

// raiseNextID moves NextID past every id in use.
func (d *Data) raiseNextID() {
	maxID := firstID - 1
	walkFolders(d.Folders, func(f *Folder) {
		if f.ID > maxID {
			maxID = f.ID
		}
	})
	for _, s := range d.Stations {
		if s.ID > maxID {
			maxID = s.ID
		}
	}
	if d.NextID <= maxID {
		d.NextID = maxID + 1
	}
}

// newID returns the next free id.
func (d *Data) newID() (int, error) {
	if d.NextID > MaxID {
		return 0, &FieldError{"catalog", "nextId", "no free ids left"}
	}
	d.NextID++
	return d.NextID - 1, nil
}

func (d *Data) validateStations(seen map[int]bool) error {
	for i, s := range d.Stations {
		name := fmt.Sprintf("station %q", s.Name)
		s.Name = CleanName(s.Name)
		s.URL = strings.TrimSpace(s.URL)
		if s.Name == "" {
			return &FieldError{fmt.Sprintf("station #%d", i+1), "name", "name is empty"}
		}
		u, err := url.Parse(s.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return &FieldError{name, "url", "URL must start with http:// or https://"}
		}
		if err := s.checkLimits(); err != nil {
			return err
		}
		if s.ID == 0 {
			if s.ID, err = d.newID(); err != nil {
				return err
			}
		}
		if seen[s.ID] {
			return &FieldError{name, "id", fmt.Sprintf("id %d is used twice", s.ID)}
		}
		seen[s.ID] = true
	}
	return nil
}

func (d *Data) validateFolders(seen map[int]bool) error {
	var first error
	walkFolders(d.Folders, func(f *Folder) {
		if first == nil {
			first = d.validateFolder(f, seen)
		}
	})
	return first
}

func (d *Data) validateFolder(f *Folder, seen map[int]bool) error {
	f.Name = CleanName(f.Name)
	if f.Name == "" {
		return &FieldError{"folder", "name", "name is empty"}
	}
	if utf8.RuneCountInString(f.Name) > maxNameLen {
		return &FieldError{"folder", "name", "name is too long"}
	}
	if f.Filter != nil {
		if len(f.Folders) > 0 || len(f.Stations) > 0 {
			return &FieldError{fmt.Sprintf("folder %q", f.Name), "filter", "a virtual folder can't hold sub-folders or stations"}
		}
		if err := f.Filter.Normalize(f.Name); err != nil {
			return err
		}
	}
	if f.ID == 0 {
		id, err := d.newID()
		if err != nil {
			return err
		}
		f.ID = id
	}
	if seen[f.ID] {
		return &FieldError{fmt.Sprintf("folder %q", f.Name), "id", fmt.Sprintf("id %d is used twice", f.ID)}
	}
	seen[f.ID] = true
	return nil
}

// checkFolderStations rejects folders that name a station that doesn't exist.
func (d *Data) checkFolderStations() error {
	stationIDs := map[int]bool{}
	for _, s := range d.Stations {
		stationIDs[s.ID] = true
	}
	var first error
	walkFolders(d.Folders, func(f *Folder) {
		for _, id := range f.Stations {
			if first == nil && !stationIDs[id] {
				first = &FieldError{fmt.Sprintf("folder %q", f.Name), "stations", fmt.Sprintf("unknown station id %d", id)}
			}
		}
	})
	return first
}

// checkFolderEntries rejects empty entries and ids out of range. It runs before anything
// else reads the folders, so the rest of the package can rely on them.
func checkFolderEntries(folders []*Folder) error {
	for _, f := range folders {
		if f == nil {
			return &FieldError{"folder", "", "empty entry"}
		}
		if f.ID < 0 || f.ID > MaxID {
			return &FieldError{fmt.Sprintf("folder %q", f.Name), "id", fmt.Sprintf("must be from 0 to %d", MaxID)}
		}
		if err := checkFolderEntries(f.Folders); err != nil {
			return err
		}
	}
	return nil
}

func (s *Station) checkLimits() error {
	entry := fmt.Sprintf("station %q", s.Name)
	for _, c := range []struct {
		field, val string
		max        int
	}{
		{"name", s.Name, maxNameLen},
		{"url", s.URL, maxURLLen},
		{"description", s.Description, maxTextLen},
		{"format", s.Format, maxShortLen},
		{"location", s.Location, maxShortLen},
		{"logo", s.Logo, maxURLLen},
		{"source", s.Source, maxShortLen},
		{"sourceId", s.SourceID, maxShortLen},
	} {
		if utf8.RuneCountInString(c.val) > c.max {
			return &FieldError{entry, c.field, fmt.Sprintf("longer than %d characters", c.max)}
		}
	}
	if s.Bitrate < 0 || s.Bitrate > maxBitrate {
		return &FieldError{entry, "bitrate", fmt.Sprintf("must be from 0 to %d", maxBitrate)}
	}
	return nil
}

func walkFolders(fs []*Folder, fn func(*Folder)) {
	for _, f := range fs {
		if f == nil {
			continue
		}
		fn(f)
		walkFolders(f.Folders, fn)
	}
}

// ErrConflict means the catalog changed since the caller read it.
var ErrConflict = errors.New("this list was changed in another window; discard your changes or reload the page, then try again")

// Store keeps the catalog in memory and on disk.
type Store struct {
	path string
	// writeMu serialises whole updates, so that disk and memory always end up with
	// the same catalog and no update works from an out-of-date copy.
	writeMu sync.Mutex
	mu      sync.RWMutex // guards the fields below
	data    *Data
	byID    map[int]*Station
	fold    map[int]*Folder
	etag    string
}

// Open loads the catalog from path. A missing file is replaced by the starter catalog.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		d := Starter()
		if err := s.Replace(d); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	d, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	s.set(d)
	return s, nil
}

// Parse decodes and validates catalog JSON.
func Parse(b []byte) (*Data, error) {
	b, err := schema.UpgradeCatalog(b)
	if err != nil {
		return nil, err
	}
	var d Data
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the catalog")
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) set(d *Data) {
	byID := map[int]*Station{}
	for _, st := range d.Stations {
		byID[st.ID] = st
	}
	fold := map[int]*Folder{}
	walkFolders(d.Folders, func(f *Folder) { fold[f.ID] = f })
	b, _ := json.Marshal(d)
	sum := sha256.Sum256(b)
	s.mu.Lock()
	s.data, s.byID, s.fold, s.etag = d, byID, fold, hex.EncodeToString(sum[:16])
	s.mu.Unlock()
}

// Snapshot returns a deep copy of the catalog.
func (s *Store) Snapshot() *Data {
	d, _ := s.SnapshotETag()
	return d
}

// SnapshotETag returns a deep copy of the catalog and the tag that identifies this version.
func (s *Store) SnapshotETag() (*Data, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data)
	var d Data
	_ = json.Unmarshal(b, &d)
	return &d, s.etag
}

// Replace validates d, keeps ids growing past the current NextID, and saves it atomically.
func (s *Store) Replace(d *Data) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.replaceLocked(d)
}

// ReplaceIf is Replace, but fails with ErrConflict when etag is not empty and is not the
// current version.
func (s *Store) ReplaceIf(etag string, d *Data) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if etag != "" && etag != s.etag {
		return ErrConflict
	}
	return s.replaceLocked(d)
}

// replaceLocked needs writeMu. Only holders of writeMu change s.data, so it can be read here.
func (s *Store) replaceLocked(d *Data) error {
	if s.data != nil && d.NextID < s.data.NextID {
		d.NextID = s.data.NextID
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := s.save(d); err != nil {
		return err
	}
	s.set(d)
	return nil
}

// Merge adds the folders and stations of in to the catalog, giving clashing entries new ids.
func (s *Store) Merge(in *Data) error {
	if err := in.Validate(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	cur := s.Snapshot()
	remap := map[int]int{}
	for _, st := range in.Stations {
		remap[st.ID] = cur.NextID
		st.ID = cur.NextID
		cur.NextID++
		cur.Stations = append(cur.Stations, st)
	}
	walkFolders(in.Folders, func(f *Folder) {
		f.ID = cur.NextID
		cur.NextID++
		for i, id := range f.Stations {
			f.Stations[i] = remap[id]
		}
	})
	cur.Folders = append(cur.Folders, in.Folders...)
	return s.replaceLocked(cur)
}

func (s *Store) save(d *Data) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	return atomicfile.Write(s.path, append(b, '\n'), 0o600)
}

// Station returns the station with id.
func (s *Store) Station(id int) (Station, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.byID[id]
	if !ok {
		return Station{}, false
	}
	return *st, true
}

// Entry is one row of a listing: a folder or a station.
type Entry struct {
	Folder     *Folder
	ChildCount int
	Station    *Station
}

// List returns the entries of folder id, or the top level when id is 0.
// The second result is false when the folder does not exist.
func (s *Store) List(id int) ([]Entry, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var folders []*Folder
	var stations []int
	name := ""
	if id == 0 {
		folders = s.data.Folders
	} else {
		f, ok := s.fold[id]
		if !ok {
			return nil, "", false
		}
		folders, stations, name = f.Folders, f.Stations, f.Name
	}
	var out []Entry
	for _, f := range folders {
		c := *f
		out = append(out, Entry{Folder: &c, ChildCount: len(f.Folders) + len(f.Stations)})
	}
	for _, sid := range stations {
		if st, ok := s.byID[sid]; ok {
			c := *st
			out = append(out, Entry{Station: &c})
		}
	}
	return out, name, true
}

// Filter returns the filter of folder id, or nil when it isn't a virtual folder.
func (s *Store) Filter(id int) *Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.fold[id]
	if !ok || f.Filter == nil {
		return nil
	}
	c := *f.Filter
	c.Genres = slices.Clone(c.Genres)
	return &c
}

// Search returns stations whose name or description contains q, ignoring case and diacritics.
func (s *Store) Search(q string) []Station {
	q = Fold(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Station
	for _, st := range s.data.Stations {
		if strings.Contains(Fold(st.Name), q) || strings.Contains(Fold(st.Description), q) {
			out = append(out, *st)
		}
	}
	return out
}

// Starter is the example catalog created on first start. All entries are fictional placeholders.
func Starter() *Data {
	return &Data{
		Version: schema.Current,
		Stations: []*Station{
			{Name: "Example Station (edit me)", URL: "http://stream.example.com/live.mp3", Description: "Placeholder. Replace it in the vTuner Radio app in DSM.", Format: "MP3", Bitrate: 128},
		},
		Folders: []*Folder{
			{Name: "My Stations", Stations: []int{firstID}},
		},
	}
}
