package directory

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"denon-synology-vtuner/internal/catalog"
	"slices"
	"sync"
)

func TestStationsFilterAndCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !strings.HasPrefix(r.URL.Path, "/json/stations/search") || r.URL.Query().Get("countrycode") != "PL" {
			t.Errorf("unexpected %s", r.URL)
		}
		w.Write([]byte(`[
		 {"stationuuid":"u1","name":" Example One ","url_resolved":"http://stream.example.com/1","codec":"MP3"},
		 {"stationuuid":"u2","name":"HLS","url_resolved":"http://stream.example.com/2.m3u8","codec":"AAC","hls":1},
		 {"stationuuid":"u3","name":"Opus","url_resolved":"http://stream.example.com/3","codec":"OGG"}]`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	got, err := c.Stations(t.Context(), Query{CountryCode: "PL"})
	if err != nil || len(got) != 1 || got[0].Name != "Example One" {
		t.Fatalf("%v %+v", err, got)
	}
	c.Stations(t.Context(), Query{CountryCode: "PL"})
	if calls != 1 {
		t.Fatalf("cache not used: %d calls", calls)
	}
	if s, ok := c.ByNumericID(got[0].NumericID()); !ok || s.UUID != "u1" {
		t.Fatal("numeric id lookup")
	}
}

func TestStationsFormatFilter(t *testing.T) {
	var gotCodec string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCodec = r.URL.Query().Get("codec")
		w.Write([]byte(`[
		 {"stationuuid":"u1","name":"One","url_resolved":"http://stream.example.com/1","codec":"MP3"},
		 {"stationuuid":"u2","name":"Two","url_resolved":"http://stream.example.com/2","codec":"AAC"},
		 {"stationuuid":"u3","name":"Three","url_resolved":"http://stream.example.com/3","codec":"AAC+"}]`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	got, err := c.Stations(t.Context(), Query{CountryCode: "PL", Codec: "aac"})
	if err != nil || len(got) != 1 || got[0].UUID != "u2" || gotCodec != "AAC" {
		t.Fatalf("AAC: %v %+v codec=%q", err, got, gotCodec)
	}
	got, err = c.Stations(t.Context(), Query{CountryCode: "PL"})
	if err != nil || len(got) != 3 {
		t.Fatalf("any format: %v %+v", err, got)
	}
	if _, err := c.Stations(t.Context(), Query{CountryCode: "PL", Codec: "OGG"}); err == nil {
		t.Fatal("unplayable format accepted")
	}
}

func TestStationsAllFilters(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query().Encode())
		w.Write([]byte(`[
		 {"stationuuid":"u0","name":"HLS","url_resolved":"http://stream.example.com/0","codec":"MP3","hls":1},
		 {"stationuuid":"u1","name":"One","url_resolved":"http://stream.example.com/1","codec":"MP3"},
		 {"stationuuid":"u2","name":"Two","url_resolved":"http://stream.example.com/2","codec":"MP3"},
		 {"stationuuid":"u3","name":"Three","url_resolved":"http://stream.example.com/3","codec":"MP3"}]`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	f := catalog.Filter{Name: "fm", Country: "PL", Genres: []string{"rock"}, Language: "polish", Format: "MP3", MinBitrate: 128, Order: "votes", Limit: 2}
	list, err := c.Stations(t.Context(), FromFilter(f, 200))
	if err != nil {
		t.Fatal(err)
	}
	// The HLS station doesn't use up one of the two places.
	if len(list) != 2 || list[0].UUID != "u1" || list[1].UUID != "u2" {
		t.Fatalf("%+v", list)
	}
	for _, want := range []string{"name=fm", "countrycode=PL", "tag=rock", "language=polish", "codec=MP3", "bitrateMin=128", "order=votes", "reverse=true", "hidebroken=true", "limit=4"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("%q has no %s", got[0], want)
		}
	}
	// Names read from A to Z, and two calls with other filters aren't served from one cache entry.
	if _, err := c.Stations(t.Context(), FromFilter(catalog.Filter{Country: "PL", Order: "name"}, 200)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[1], "order=name") || !strings.Contains(got[1], "reverse=false") {
		t.Fatalf("%v", got)
	}
}

func TestFromFilterLimit(t *testing.T) {
	for _, c := range []struct{ filter, max, want int }{{0, 200, 200}, {20, 200, 20}, {500, 200, 200}, {20, 0, 20}, {0, 0, 0}} {
		if q := FromFilter(catalog.Filter{Limit: c.filter}, c.max); q.Limit != c.want {
			t.Errorf("filter %d, max %d: got %d, want %d", c.filter, c.max, q.Limit, c.want)
		}
	}
}

func TestNamelessEntriesAreDropped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/tags", "/json/languages":
			w.Write([]byte(`[{"name":"","stationcount":5},{"name":"  ","stationcount":5},{"name":"\u200b","stationcount":5},{"name":" rock ","stationcount":5}]`))
		case "/json/stations/search":
			w.Write([]byte(`[{"stationuuid":"u1","name":" ","url_resolved":"http://stream.example.com/1","codec":"MP3"},
			 {"stationuuid":"u2","name":"\u200b","url_resolved":"http://stream.example.com/2","codec":"MP3"},
			 {"stationuuid":"u3","name":"Named","url_resolved":"http://stream.example.com/3","codec":"MP3"}]`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	for what, call := range map[string]func(context.Context, int) ([]Tag, error){"tags": c.Tags, "languages": c.Languages} {
		got, err := call(t.Context(), 10)
		if err != nil || len(got) != 1 || got[0].Name != "rock" {
			t.Fatalf("%s: %v %+v", what, err, got)
		}
	}
	list, err := c.Stations(t.Context(), Query{Tag: "rock"})
	if err != nil || len(list) != 1 || list[0].Name != "Named" {
		t.Fatalf("%v %+v", err, list)
	}
}

func TestCleanCountries(t *testing.T) {
	got := cleanCountries([]Country{
		{Name: "Poland", Code: "PL", StationCount: 10},
		{Name: "", Code: "XX", StationCount: 1},
		{Name: " ", Code: "UN", StationCount: 1},
		{Name: "\u200b", Code: "ZZ", StationCount: 1},
		{Name: "Türkiye", Code: "TR", StationCount: 5},
		{Name: "Türkiye", Code: "tr", StationCount: 2},
		{Name: "Türkiye", Code: "QT", StationCount: 1},
		{Name: "The United Kingdom Of Great Britain And Northern Ireland", Code: "GB", StationCount: 7},
		{Name: "Bolivia (Plurinational State Of)", Code: "BX", StationCount: 3},
		{Name: "Nowhere", Code: "NW", StationCount: 0},
	})
	var names []string
	for _, c := range got {
		names = append(names, fmt.Sprintf("%s/%s/%d", c.Name, c.Code, c.StationCount))
	}
	want := "Bolivia/BX/3,Poland/PL/10,Türkiye/TR/7,United Kingdom/GB/7"
	if strings.Join(names, ",") != want {
		t.Fatalf("got  %s\nwant %s", strings.Join(names, ","), want)
	}
}

func TestDiscoveryFailureIsNotRemembered(t *testing.T) {
	c := New("")
	var found []string
	calls := 0
	c.discover = func(context.Context) []string { calls++; return found }

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if got := c.bases(cancelled); len(got) != 1 || got[0] != FallbackServer {
		t.Fatal(got)
	}
	// A caller that gave up says nothing about DNS, so the next call looks again.
	found = []string{"https://a.example.com", "https://b.example.com"}
	if got := c.bases(t.Context()); len(got) != 2 {
		t.Fatalf("still pinned: %v", got)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
	// The list is reused without another lookup.
	c.bases(t.Context())
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestDiscoveryKeepsKnownServersWhenItFails(t *testing.T) {
	c := New("")
	c.discover = func(context.Context) []string { return []string{"https://a.example.com"} }
	c.bases(t.Context())
	c.discover = func(context.Context) []string { return nil }
	c.nextDiscover = time.Time{} // the list is due for a refresh
	if got := c.bases(t.Context()); len(got) != 1 || got[0] != "https://a.example.com" {
		t.Fatal(got)
	}
	if c.nextDiscover.IsZero() {
		t.Fatal("a failed lookup should wait before trying again")
	}
}

// A list of 80s and 90s holds stations with either genre, once each, in the folder's order.
func TestStationsWithAnyGenre(t *testing.T) {
	rows := map[string]string{
		"80s": `[{"stationuuid":"a","name":"Alpha","url_resolved":"http://stream.example.com/a","codec":"MP3","clickcount":50,"votes":1,"bitrate":64},
			{"stationuuid":"both","name":"Both","url_resolved":"http://stream.example.com/b","codec":"MP3","clickcount":30,"votes":9,"bitrate":128}]`,
		"90s": `[{"stationuuid":"c","name":"Charlie","url_resolved":"http://stream.example.com/c","codec":"MP3","clickcount":40,"votes":5,"bitrate":320},
			{"stationuuid":"both","name":"Both","url_resolved":"http://stream.example.com/b","codec":"MP3","clickcount":30,"votes":9,"bitrate":128}]`,
	}
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		mu.Lock()
		asked = append(asked, tag)
		mu.Unlock()
		if body, ok := rows[tag]; ok {
			w.Write([]byte(body))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ids := func(f catalog.Filter) string {
		t.Helper()
		list, err := c.Stations(t.Context(), FromFilter(f, 200))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, s := range list {
			out = append(out, s.UUID)
		}
		return strings.Join(out, ",")
	}
	genres := []string{"80s", "90s"}
	for _, x := range []struct {
		f    catalog.Filter
		want string
	}{
		{catalog.Filter{Genres: genres}, "a,c,both"},
		{catalog.Filter{Genres: genres, Order: "votes"}, "both,c,a"},
		{catalog.Filter{Genres: genres, Order: "bitrate"}, "c,both,a"},
		{catalog.Filter{Genres: genres, Order: "name"}, "a,both,c"},
		{catalog.Filter{Genres: genres, Limit: 2}, "a,c"},
	} {
		if got := ids(x.f); got != x.want {
			t.Errorf("%+v: got %s, want %s", x.f, got, x.want)
		}
	}
	if slices.Contains(asked, "80s,90s") || !slices.Contains(asked, "80s") || !slices.Contains(asked, "90s") {
		t.Errorf("asked %v", asked)
	}
	// One genre that can't be read fails the listing, so it never shows a part as the whole.
	if _, err := c.Stations(t.Context(), FromFilter(catalog.Filter{Genres: []string{"80s", "bad"}}, 200)); err == nil {
		t.Error("no error when one genre fails")
	}
}

// The cache stays within its byte budget, dropping the oldest answers first.
func TestCacheSizeIsBounded(t *testing.T) {
	defer func(n int) { maxCacheBytes = n }(maxCacheBytes)
	maxCacheBytes = 100
	c := New("http://directory.example.com")
	for i := range 10 {
		c.store(fmt.Sprintf("/p%d", i), make([]byte, 30))
		time.Sleep(time.Millisecond)
	}
	if c.cacheBytes > maxCacheBytes || len(c.cache) != 3 {
		t.Fatalf("%d bytes in %d entries", c.cacheBytes, len(c.cache))
	}
	for _, p := range []string{"/p7", "/p8", "/p9"} {
		if _, ok := c.cache[p]; !ok {
			t.Errorf("newest answer %s dropped", p)
		}
	}
	c.store("/p9", make([]byte, 10)) // replacing an entry doesn't count it twice
	if c.cacheBytes != 70 {
		t.Fatalf("%d bytes after replacing", c.cacheBytes)
	}
	c.store("/huge", make([]byte, 200))
	if _, ok := c.cache["/huge"]; ok || c.cacheBytes != 70 {
		t.Fatalf("an answer bigger than the cache was kept: %d bytes", c.cacheBytes)
	}
}
