package vtuner

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"denon-synology-vtuner/internal/catalog"
	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/directory"
	"denon-synology-vtuner/internal/relay"
	"denon-synology-vtuner/internal/reqlog"
)

type list struct {
	Count int `xml:"ItemCount"`
	Items []struct {
		Type      string `xml:"ItemType"`
		Title     string `xml:"Title"`
		UrlDir    string `xml:"UrlDir"`
		DirCount  int    `xml:"DirCount"`
		ID        int    `xml:"StationId"`
		Name      string `xml:"StationName"`
		StreamURL string `xml:"StationUrl"`
		Display   string `xml:"Display"`
		SearchURL string `xml:"SearchURL"`
	} `xml:"Item"`
}

func setup(t *testing.T, d *catalog.Data) *Handler {
	t.Helper()
	reg, err := devices.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if d != nil {
		s, _ := reg.Store(devices.DefaultKey)
		if err := s.Replace(d); err != nil {
			t.Fatal(err)
		}
	}
	return &Handler{Devices: reg, Log: reqlog.New(10), DefaultHost: "denon.vtuner.com", MaxTitle: 40, SearchEnabled: true, Token: "0000000000000000", DirLimit: 100, TagCount: 50}
}

func get(t *testing.T, h *Handler, host, target string) (string, list) {
	t.Helper()
	r := httptest.NewRequest("GET", target, nil)
	r.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("%s: status %d", target, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/xml") {
		t.Fatalf("content type %q", ct)
	}
	if w.Header().Get("Content-Length") == "" {
		t.Fatal("missing Content-Length")
	}
	var l list
	body := w.Body.String()
	if !strings.Contains(body, "EncryptedToken") {
		if err := xml.Unmarshal([]byte(body), &l); err != nil {
			t.Fatalf("bad xml: %v\n%s", err, body)
		}
	}
	return body, l
}

func sample(n int) *catalog.Data {
	d := &catalog.Data{}
	jazz := &catalog.Folder{ID: 2, Name: "Jazz"}
	for i := 0; i < n; i++ {
		id := 1000 + i
		d.Stations = append(d.Stations, &catalog.Station{ID: id, Name: fmt.Sprintf("Example Station %02d", i+1), URL: "http://stream.example.com/" + fmt.Sprint(i)})
		jazz.Stations = append(jazz.Stations, id)
	}
	d.Folders = []*catalog.Folder{{ID: 1, Name: "News"}, jazz}
	return d
}

const login = "/setupapp/denon/asp/BrowseXML/loginXML.asp"

func TestToken(t *testing.T) {
	h := setup(t, nil)
	body, _ := get(t, h, "denon.vtuner.com", login+"?token=0")
	if body != "<EncryptedToken>0000000000000000</EncryptedToken>" {
		t.Fatal(body)
	}
}

func TestMixedCasePath(t *testing.T) {
	h := setup(t, nil)
	body, _ := get(t, h, "denon.vtuner.com", "/setupapp/Denon/asp/browsexml/LOGINXML.ASP?Token=0")
	if !strings.Contains(body, "EncryptedToken") {
		t.Fatal(body)
	}
}

func TestRootMenuOrderAndSearch(t *testing.T) {
	h := setup(t, sample(2))
	_, l := get(t, h, "denon.vtuner.com", login+"?mac=001122334455&dlang=eng&fver=1")
	if len(l.Items) != 3 || l.Items[0].Title != "News" || l.Items[1].Title != "Jazz" || l.Items[1].DirCount != 2 || l.Items[2].Type != "Search" {
		t.Fatalf("%+v", l)
	}
	if !strings.HasPrefix(l.Items[1].UrlDir, "http://denon.vtuner.com/setupapp/denon/") {
		t.Fatal(l.Items[1].UrlDir)
	}
	if e := h.Log.Recent()[0]; !strings.Contains(e.Query, "mac=001122334455") {
		t.Fatal("login parameters not logged")
	}
}

func TestOtherVTunerHost(t *testing.T) {
	h := setup(t, sample(1))
	_, l := get(t, h, "example.vtuner.com", login)
	if !strings.HasPrefix(l.Items[0].UrlDir, "http://example.vtuner.com/") {
		t.Fatal(l.Items[0].UrlDir)
	}
}

func TestEmptyCatalog(t *testing.T) {
	h := setup(t, &catalog.Data{})
	_, l := get(t, h, "denon.vtuner.com", login)
	if l.Items[0].Type != "Display" {
		t.Fatalf("%+v", l)
	}
}

func TestFolderPaging(t *testing.T) {
	h := setup(t, sample(25))
	nav := "/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=folder&id=2&dev=default"
	_, l := get(t, h, "denon.vtuner.com", nav+"&startItems=11&endItems=20")
	if l.Count != 25 || len(l.Items) != 10 || l.Items[0].Name != "Example Station 11" {
		t.Fatalf("count %d items %d", l.Count, len(l.Items))
	}
	_, l = get(t, h, "denon.vtuner.com", nav+"&startItems=21&endItems=40")
	if len(l.Items) != 5 {
		t.Fatalf("items %d", len(l.Items))
	}
}

func TestStationLookupAndRelayURL(t *testing.T) {
	d := sample(2)
	d.Stations[1].Relay = true
	h := setup(t, d)
	_, l := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/statxml.asp?id=1000")
	if len(l.Items) != 1 || l.Items[0].StreamURL != "http://stream.example.com/0" {
		t.Fatalf("%+v", l)
	}
	_, l = get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/statxml.asp?id=1001")
	if l.Items[0].StreamURL != "http://denon.vtuner.com/relay/default/1001" {
		t.Fatal(l.Items[0].StreamURL)
	}
	_, l = get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/statxml.asp?id=9999")
	if len(l.Items) != 0 || h.Log.Recent()[0].Note == "" {
		t.Fatal("unknown station should give empty list and a log note")
	}
}

func TestSearch(t *testing.T) {
	h := setup(t, sample(3))
	_, l := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/Search.asp?sSearchtype=2&search=station%2002")
	if len(l.Items) != 1 {
		t.Fatalf("%+v", l)
	}
	_, l = get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/Search.asp?sSearchtype=2&search=zzz")
	if len(l.Items) != 0 {
		t.Fatal("want empty")
	}
}

func TestEscapingAndShortening(t *testing.T) {
	d := &catalog.Data{Stations: []*catalog.Station{
		{ID: 1000, Name: "Rock & Roll Example", URL: "http://stream.example.com/a"},
		{ID: 1001, Name: strings.Repeat("Very long name ", 10), URL: "http://stream.example.com/b"},
	}, Folders: []*catalog.Folder{{ID: 1, Name: "F", Stations: []int{1000, 1001}}}}
	h := setup(t, d)
	body, l := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=folder&id=1")
	if !strings.Contains(body, "Rock &amp; Roll Example") {
		t.Fatal(body)
	}
	if n := len([]rune(l.Items[1].Name)); n > 40 {
		t.Fatalf("name not shortened: %d", n)
	}
}

func TestUnknownPath(t *testing.T) {
	h := setup(t, sample(1))
	_, l := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/unknown.asp")
	if len(l.Items) != 0 || h.Log.Recent()[0].Path != "/setupapp/denon/asp/BrowseXML/unknown.asp" {
		t.Fatal("unknown path")
	}
}

func TestIsVTunerHost(t *testing.T) {
	for h, want := range map[string]bool{"denon.vtuner.com": true, "DENON.VTUNER.COM:80": true, "vtuner.com": true, "nas.example.lan": false, "evilvtuner.com": false} {
		if IsVTunerHost(h) != want {
			t.Errorf("%s", h)
		}
	}
}

func TestPerReceiverLists(t *testing.T) {
	h := setup(t, sample(2))
	// receiver A logs in and gets a copy of the default list
	_, l := get(t, h, "denon.vtuner.com", login+"?mac=00:11:22:33:44:55&dlang=eng")
	if !strings.Contains(l.Items[0].UrlDir, "dev=001122334455") {
		t.Fatal(l.Items[0].UrlDir)
	}
	sa, _ := h.Devices.Store("001122334455")
	d := sa.Snapshot()
	d.Folders = d.Folders[:1]
	sa.Replace(d)
	_, la := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=root&dev=001122334455")
	_, lb := get(t, h, "denon.vtuner.com", login+"?mac=001122334466")
	if len(la.Items) == len(lb.Items) {
		t.Fatalf("lists should differ: %d vs %d", len(la.Items), len(lb.Items))
	}
}

func TestRelayResolver(t *testing.T) {
	d := sample(2)
	d.Stations[1].Relay = true
	h := setup(t, d)
	if u, ok := h.RelayResolver(t.Context(), "default/1001"); !ok || u.URL != "http://stream.example.com/1" || u.Public {
		t.Fatal(u, ok)
	}
	for _, k := range []string{"default/1000", "default/9999", "../x/1", "1001", "rb/x"} {
		if _, ok := h.RelayResolver(t.Context(), k); ok {
			t.Fatalf("%s should be refused", k)
		}
	}
}

func TestDirectoryBrowsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/countries":
			w.Write([]byte(`[{"name":"Poland","iso_3166_1":"PL","stationcount":3},{"name":"","iso_3166_1":"XX","stationcount":1}]`))
		case "/json/tags":
			w.Write([]byte(`[{"name":"jazz","stationcount":9}]`))
		case "/json/stations/search":
			w.Write([]byte(`[{"stationuuid":"u1","name":"Example PL","url_resolved":"https://stream.example.com/pl","codec":"MP3","country":"Poland"}]`))
		}
	}))
	defer srv.Close()
	h := setup(t, sample(1))
	h.Directory = directory.New(srv.URL)
	_, root := get(t, h, "denon.vtuner.com", login)
	var titles []string
	for _, it := range root.Items {
		if it.Title == "" {
			it.Title = it.Type
		}
		titles = append(titles, it.Title)
	}
	if strings.Join(titles, ",") != "News,Jazz,Browse by country,Browse by genre,Search" {
		t.Fatal(titles)
	}
	if c, g := root.Items[2].DirCount, root.Items[3].DirCount; c != 1 || g != 1 {
		t.Fatalf("browse menu counts: countries %d, genres %d", c, g)
	}
	_, cs := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=countries&dev=default")
	if len(cs.Items) != 1 || !strings.Contains(cs.Items[0].UrlDir, "cc=PL") {
		t.Fatalf("%+v", cs)
	}
	_, st := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=country&cc=PL&dev=default")
	if len(st.Items) != 1 || st.Items[0].StreamURL != "http://denon.vtuner.com/relay/rb/u1" {
		t.Fatalf("%+v", st)
	}
	_, se := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/Search.asp?sSearchtype=2&search=example")
	if len(se.Items) != 1 || se.Items[0].Name != "Example Station 01" {
		t.Fatalf("search should list only the receiver's own stations: %+v", se)
	}
}

// A public directory entry can name any URL, so the relay must not follow it into the NAS.
func TestDirectoryRelayCantReachLoopback(t *testing.T) {
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("reached the loopback server: %s", r.URL)
	}))
	defer inner.Close()
	const uuid = "00000000-0000-4000-8000-000000000001"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"stationuuid":%q,"name":"x","url_resolved":%q}]`, uuid, inner.URL+"/secret")
	}))
	defer api.Close()
	h := setup(t, sample(1))
	h.Directory = directory.New(api.URL)
	target, ok := h.RelayResolver(t.Context(), "rb/"+uuid)
	if !ok || !target.Public {
		t.Fatalf("%+v %v", target, ok)
	}
	if _, ok := h.RelayResolver(t.Context(), "rb/not-a-uuid"); ok {
		t.Fatal("malformed uuid accepted")
	}
	w := httptest.NewRecorder()
	relay.New(h.RelayResolver, 1).ServeHTTP(w, httptest.NewRequest("GET", "/relay/rb/"+uuid, nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestListStationFromDirectoryIsPublicTarget(t *testing.T) {
	d := sample(1)
	d.Stations[0].Relay, d.Stations[0].Source, d.Stations[0].SourceID = true, "radio-browser", "u1"
	h := setup(t, d)
	if u, ok := h.RelayResolver(t.Context(), fmt.Sprintf("default/%d", d.Stations[0].ID)); !ok || !u.Public {
		t.Fatalf("%+v %v", u, ok)
	}
}

func virtualSample() *catalog.Data {
	return &catalog.Data{Folders: []*catalog.Folder{
		{Name: "Polish rock", Filter: &catalog.Filter{Country: "PL", Genres: []string{"rock"}, Format: "MP3", Name: "fm"}},
	}}
}

func TestVirtualFolder(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/stations/search":
			got = append(got, r.URL.RawQuery)
			w.Write([]byte(`[{"stationuuid":"u1","name":"Example Rock FM","url_resolved":"http://stream.example.com/rock","codec":"MP3"},{"stationuuid":"u2","name":"Example Rock AAC","url_resolved":"http://stream.example.com/aac","codec":"AAC"},{"stationuuid":"u3","name":"Example Secure FM","url_resolved":"https://stream.example.com/secure","codec":"MP3"}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()
	h := setup(t, virtualSample())
	h.Directory = directory.New(srv.URL)
	s, _ := h.Devices.Store(devices.DefaultKey)
	id := s.Snapshot().Folders[0].ID

	_, root := get(t, h, "denon.vtuner.com", login)
	if root.Items[0].Title != "Polish rock" || root.Items[0].DirCount != 2 {
		t.Fatalf("%+v", root.Items[0])
	}
	_, l := get(t, h, "denon.vtuner.com", fmt.Sprintf("/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=folder&id=%d&dev=default&startitems=1&enditems=10", id))
	if l.Count != 2 || len(l.Items) != 2 || l.Items[0].Name != "Example Rock FM" {
		t.Fatalf("%+v", l)
	}
	if l.Items[0].StreamURL != "http://stream.example.com/rock" || l.Items[1].StreamURL != "http://denon.vtuner.com/relay/rb/u3" {
		t.Fatalf("%+v", l.Items)
	}
	if len(got) == 0 {
		t.Fatal("the directory wasn't asked")
	}
	for _, want := range []string{"countrycode=PL", "tag=rock", "name=fm", "codec=MP3"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("query %q has no %s", got[0], want)
		}
	}
	// A station from the listing can be looked up by its id, like any directory station.
	_, st := get(t, h, "denon.vtuner.com", fmt.Sprintf("/setupapp/denon/asp/BrowseXML/statXML.asp?id=%d&dev=default", l.Items[0].ID))
	if len(st.Items) != 1 || st.Items[0].Name != "Example Rock FM" {
		t.Fatalf("%+v", st)
	}
}

func TestVirtualFolderWithoutDirectoryIsEmpty(t *testing.T) {
	open := func(h *Handler) list {
		s, _ := h.Devices.Store(devices.DefaultKey)
		id := s.Snapshot().Folders[0].ID
		_, l := get(t, h, "denon.vtuner.com", fmt.Sprintf("/setupapp/denon/asp/BrowseXML/navXML.asp?gofile=folder&id=%d&dev=default", id))
		return l
	}
	h := setup(t, virtualSample())
	if l := open(h); l.Count != 0 || len(l.Items) != 0 {
		t.Fatalf("directory off: %+v", l)
	}
	if note := h.Log.Recent()[0].Note; !strings.Contains(note, "disabled") {
		t.Fatalf("no reason in the log: %q", note)
	}
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) }))
	defer empty.Close()
	h = setup(t, virtualSample())
	h.Directory = directory.New(empty.URL)
	if l := open(h); l.Count != 0 || len(l.Items) != 0 {
		t.Fatalf("no match: %+v", l)
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer down.Close()
	h = setup(t, virtualSample())
	h.Directory = directory.New(down.URL)
	_, root := get(t, h, "denon.vtuner.com", login)
	if root.Items[0].Title != "Polish rock" || root.Items[0].DirCount != 0 {
		t.Fatalf("root with the directory down: %+v", root.Items[0])
	}
	if l := open(h); l.Count != 0 || len(l.Items) != 0 {
		t.Fatalf("directory down: %+v", l)
	}
	if note := h.Log.Recent()[0].Note; !strings.Contains(note, "directory:") {
		t.Fatalf("no reason in the log: %q", note)
	}
}

func TestVirtualFolderFiltersReachTheDirectory(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/stations/search" {
			w.Write([]byte(`[]`))
			return
		}
		got = append(got, r.URL.RawQuery)
		var rows []string
		for i := range 30 {
			rows = append(rows, fmt.Sprintf(`{"stationuuid":"u%d","name":"Example %d","url_resolved":"http://stream.example.com/%d","codec":"MP3"}`, i, i, i))
		}
		w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	}))
	defer srv.Close()
	h := setup(t, &catalog.Data{Folders: []*catalog.Folder{{ID: 1, Name: "Parent", Folders: []*catalog.Folder{
		{ID: 2, Name: "Polish talk", Filter: &catalog.Filter{Country: "PL", Language: "Polish", MinBitrate: 128, Order: "votes", Limit: 5}},
	}}}})
	h.Directory = directory.New(srv.URL)
	nav := "/setupapp/denon/asp/BrowseXML/navXML.asp?dev=default&gofile=folder&id="
	_, parent := get(t, h, "denon.vtuner.com", nav+"1")
	if len(parent.Items) != 1 || parent.Items[0].Type != "Dir" || parent.Items[0].Title != "Polish talk" || parent.Items[0].DirCount != 5 {
		t.Fatalf("%+v", parent)
	}
	_, l := get(t, h, "denon.vtuner.com", nav+"2&startItems=4&endItems=20")
	if l.Count != 5 || len(l.Items) != 2 {
		t.Fatalf("max results not kept: %+v", l)
	}
	for _, want := range []string{"countrycode=PL", "language=polish", "bitrateMin=128", "order=votes", "reverse=true", "hidebroken=true", "limit=10"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("query %q has no %s", got[0], want)
		}
	}
}

// A folder with two genres counts and lists the stations of both, and pages stay in step.
func TestVirtualFolderWithSeveralGenres(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		var rows []string
		for i := range 3 {
			rows = append(rows, fmt.Sprintf(`{"stationuuid":"%s%d","name":"%s %d","url_resolved":"http://stream.example.com/%s%d","codec":"MP3"}`, tag, i, tag, i, tag, i))
		}
		w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	}))
	defer srv.Close()
	h := setup(t, &catalog.Data{Folders: []*catalog.Folder{{ID: 1, Name: "Parent", Folders: []*catalog.Folder{
		{ID: 2, Name: "80s and 90s", Filter: &catalog.Filter{Genres: []string{"80s", "90s"}}},
	}}}})
	h.Directory = directory.New(srv.URL)
	nav := "/setupapp/denon/asp/BrowseXML/navXML.asp?dev=default&gofile=folder&id="
	_, top := get(t, h, "denon.vtuner.com", nav+"1")
	if len(top.Items) == 0 || top.Items[0].Title != "80s and 90s" || top.Items[0].DirCount != 6 {
		t.Fatalf("%+v", top)
	}
	_, l := get(t, h, "denon.vtuner.com", nav+"2&startItems=1&endItems=4")
	if l.Count != 6 || len(l.Items) != 4 {
		t.Fatalf("%+v", l)
	}
	_, l2 := get(t, h, "denon.vtuner.com", nav+"2&startItems=5&endItems=8")
	if l2.Count != 6 || len(l2.Items) != 2 {
		t.Fatalf("%+v", l2)
	}
}

// walkMenus opens every menu the receiver can reach from the login answer and calls fn on each.
func walkMenus(t *testing.T, h *Handler, start string, fn func(url string, l list)) {
	t.Helper()
	seen := map[string]bool{}
	var open func(u string)
	open = func(u string) {
		if seen[u] {
			return
		}
		seen[u] = true
		_, l := get(t, h, "denon.vtuner.com", u)
		fn(u, l)
		for _, it := range l.Items {
			if it.Type == "Dir" {
				open(it.UrlDir)
			}
		}
	}
	open(start)
}

func checkAllNamed(t *testing.T, u string, l list) {
	t.Helper()
	for _, it := range l.Items {
		text := it.Title
		if it.Type == "Station" {
			text = it.Name
		}
		if catalog.Blank(text) {
			t.Errorf("%s: %s item has no name: %+v", u, it.Type, it)
		}
	}
}

func TestNoMenuHasARowWithoutAName(t *testing.T) {
	blank := []string{"", " ", "\t", "\u00a0", "\u200b"}
	var countries, tags, stations []string
	for i, b := range blank {
		countries = append(countries, fmt.Sprintf(`{"name":%q,"iso_3166_1":"X%d","stationcount":2}`, b, i))
		tags = append(tags, fmt.Sprintf(`{"name":%q,"stationcount":2}`, b))
		stations = append(stations, fmt.Sprintf(`{"stationuuid":"b%d","name":%q,"url_resolved":"http://stream.example.com/b","codec":"MP3"}`, i, b))
	}
	countries = append(countries, `{"name":"Poland","iso_3166_1":"PL","stationcount":3}`)
	tags = append(tags, `{"name":"jazz","stationcount":9}`)
	stations = append(stations, `{"stationuuid":"u1","name":"Example FM","url_resolved":"http://stream.example.com/1","codec":"MP3"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rows := map[string][]string{"/json/countries": countries, "/json/tags": tags, "/json/stations/search": stations}[r.URL.Path]
		w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	}))
	defer srv.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer down.Close()
	data := func() *catalog.Data {
		d := sample(2)
		d.Folders = append(d.Folders, &catalog.Folder{ID: 3, Name: "Empty but named"},
			&catalog.Folder{ID: 4, Name: "Virtual", Filter: &catalog.Filter{Genres: []string{"jazz"}}})
		return d
	}
	// The Denon RCD-N7 logs in on browsexm2; the community notes use BrowseXML.
	const real = "/setupapp/denon/asp/browsexm2/loginXML.asp?dev=default&dlang=eng"
	for name, dirURL := range map[string]string{"directory on": srv.URL, "directory down": down.URL, "directory off": ""} {
		for _, search := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, search %v", name, search), func(t *testing.T) {
				h := setup(t, data())
				h.SearchEnabled = search
				if dirURL != "" {
					h.Directory = directory.New(dirURL)
				}
				menus, stationsListed := 0, 0
				emptyFolderKept := false
				walkMenus(t, h, real, func(u string, l list) {
					menus++
					checkAllNamed(t, u, l)
					if l.Count != len(l.Items) {
						t.Errorf("%s: count %d, items %d", u, l.Count, len(l.Items))
					}
					for _, it := range l.Items {
						emptyFolderKept = emptyFolderKept || it.Title == "Empty but named"
						if it.Type == "Station" {
							stationsListed++
						}
					}
				})
				if !emptyFolderKept {
					t.Error("a folder with a name and no stations must stay")
				}
				if menus < 4 || stationsListed < 2 {
					t.Errorf("walked too little: %d menus, %d stations", menus, stationsListed)
				}
			})
		}
	}
}

func TestEmptyCatalogMessageHasAName(t *testing.T) {
	h := setup(t, &catalog.Data{})
	h.SearchEnabled = false
	_, l := get(t, h, "denon.vtuner.com", login)
	if len(l.Items) != 1 || l.Items[0].Type != "Display" || catalog.Blank(l.Items[0].Title) {
		t.Fatalf("%+v", l)
	}
}

func TestNamedLeavesOutBlankItems(t *testing.T) {
	got := named([]item{{kind: "Dir", title: "\u200b "}, {kind: "Station", name: " "}, {kind: "Dir", title: "Jazz"}, {kind: "Display"}})
	if len(got) != 1 || got[0].title != "Jazz" {
		t.Fatalf("%+v", got)
	}
}

func TestShortNeverGivesABlankName(t *testing.T) {
	h := &Handler{MaxTitle: 10}
	if got := h.short(strings.Repeat(" ", 20) + "x"); catalog.Blank(got) {
		t.Fatalf("%q", got)
	}
}

func TestRootMenuHasNoNamelessRowWithSearchOff(t *testing.T) {
	h := setup(t, sample(1))
	h.SearchEnabled = false
	_, l := get(t, h, "denon.vtuner.com", login)
	for _, it := range l.Items {
		if it.Type == "Search" || it.Title == "" {
			t.Fatalf("%+v", l)
		}
	}
	// The request itself still works for a receiver that sends it.
	_, se := get(t, h, "denon.vtuner.com", "/setupapp/denon/asp/BrowseXML/Search.asp?sSearchtype=2&search=example")
	if len(se.Items) == 0 {
		t.Fatalf("%+v", se)
	}
}
