package admin

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/directory"
	"denon-synology-vtuner/internal/relay"
	"denon-synology-vtuner/internal/reqlog"
)

func setup(t *testing.T) *Handler {
	reg, err := devices.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{Devices: reg, Log: reqlog.New(5), Relay: relay.New(nil, 1), Token: "test-token"}
}

func do(h *Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set(TokenHeader, token)
	}
	if method != "GET" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTokenRequired(t *testing.T) {
	h := setup(t)
	if w := do(h, "GET", "/admin/api/catalog", "", ""); w.Code != 403 {
		t.Fatalf("no token: %d", w.Code)
	}
	if w := do(h, "GET", "/admin/api/catalog", "", "wrong"); w.Code != 403 {
		t.Fatalf("wrong token: %d", w.Code)
	}
	if w := do(h, "GET", "/admin/api/catalog", "", "test-token"); w.Code != 200 {
		t.Fatalf("good token: %d", w.Code)
	}
}

func TestPutValidates(t *testing.T) {
	h := setup(t)
	w := do(h, "PUT", "/admin/api/catalog", `{"stations":[{"name":"x","url":"ftp://example.com/s"}],"folders":[]}`, "test-token")
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"field": "url"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = do(h, "PUT", "/admin/api/catalog", `{"stations":[{"name":"x","url":"http://stream.example.com/s"}],"folders":[]}`, "test-token")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestImportInvalidChangesNothing(t *testing.T) {
	h := setup(t)
	store, _ := h.Devices.Store(devices.DefaultKey)
	before := len(store.Snapshot().Stations)
	w := do(h, "POST", "/admin/api/import?mode=merge", `{"stations":[{"name":"ok","url":"http://a.example.com"},{"name":"","url":"http://b.example.com"}],"folders":[]}`, "test-token")
	if w.Code != 400 || len(store.Snapshot().Stations) != before {
		t.Fatalf("%d", w.Code)
	}
}

func TestUIServed(t *testing.T) {
	h := setup(t)
	w := do(h, "GET", "/admin/", "", "test-token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("%d", w.Code)
	}
}

func TestDevicesAPI(t *testing.T) {
	h := setup(t)
	h.Devices.Login("00:11:22:33:44:55", "192.168.1.50")
	if w := do(h, "PATCH", "/admin/api/devices/001122334455", `{"name":"Living room"}`, "test-token"); w.Code != 200 || !strings.Contains(w.Body.String(), "Living room") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if w := do(h, "GET", "/admin/api/catalog?device=001122334455", "", "test-token"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := do(h, "GET", "/admin/api/catalog?device=../etc", "", "test-token"); w.Code != 404 {
		t.Fatal("bad key accepted")
	}
	if w := do(h, "DELETE", "/admin/api/devices/default", "", "test-token"); w.Code != 400 {
		t.Fatal("default deleted")
	}
	if w := do(h, "POST", "/admin/api/devices/001122334455/copy?from=default", "", "test-token"); w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestBodyMustBeJSON(t *testing.T) {
	h := setup(t)
	before := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String()
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", ""} {
		r := httptest.NewRequest("POST", "/admin/api/import?mode=replace", strings.NewReader(`{"stations":[],"folders":[]}`))
		r.Header.Set(TokenHeader, "test-token")
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("%q: %d", ct, w.Code)
		}
	}
	if after := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String(); after != before {
		t.Fatal("catalog changed")
	}
}

func TestImportModeMustBeKnown(t *testing.T) {
	h := setup(t)
	before := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String()
	for _, q := range []string{"", "?mode=typo"} {
		if w := do(h, "POST", "/admin/api/import"+q, `{"stations":[],"folders":[]}`, "test-token"); w.Code != 400 {
			t.Fatalf("%q: %d", q, w.Code)
		}
	}
	if after := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String(); after != before {
		t.Fatal("catalog changed")
	}
}

func TestNullEntriesRefused(t *testing.T) {
	h := setup(t)
	for _, body := range []string{`{"stations":[null],"folders":[]}`, `{"stations":[],"folders":[null]}`} {
		if w := do(h, "PUT", "/admin/api/catalog", body, "test-token"); w.Code != 400 {
			t.Fatalf("%s: %d", body, w.Code)
		}
	}
}

func TestSaveFromOutOfDateCopyRefused(t *testing.T) {
	h := setup(t)
	put := func(etag, name string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "/admin/api/catalog", strings.NewReader(`{"stations":[{"name":"`+name+`","url":"http://stream.example.com/s"}],"folders":[]}`))
		r.Header.Set(TokenHeader, "test-token")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("If-Match", etag)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	first := do(h, "GET", "/admin/api/catalog", "", "test-token").Header().Get("ETag")
	if first == "" {
		t.Fatal("no ETag")
	}
	w := put(first, "tab one")
	if w.Code != 200 || w.Header().Get("ETag") == first {
		t.Fatalf("%d %q", w.Code, w.Header().Get("ETag"))
	}
	if w := put(first, "tab two"); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("second tab: %d", w.Code)
	}
	if got := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String(); !strings.Contains(got, "tab one") {
		t.Fatal(got)
	}
}

func TestDirectoryAPI(t *testing.T) {
	h := setup(t)
	if w := do(h, "GET", "/admin/api/directory/countries", "", "test-token"); w.Code != 404 {
		t.Fatalf("directory off: %d", w.Code)
	}
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch r.URL.Path {
		case "/json/countries":
			w.Write([]byte(`[{"name":"Poland","iso_3166_1":"PL","stationcount":3}]`))
		case "/json/tags":
			w.Write([]byte(`[{"name":"jazz","stationcount":9}]`))
		default:
			w.Write([]byte(`[{"stationuuid":"u1","name":"One","url_resolved":"http://stream.example.com/1","codec":"MP3"}]`))
		}
	}))
	defer srv.Close()
	h.Directory = directory.New(srv.URL)
	for path, want := range map[string]string{
		"countries":           "Poland",
		"tags":                "jazz",
		"stations?country=PL": "stationuuid",
	} {
		if w := do(h, "GET", "/admin/api/directory/"+path, "", "test-token"); w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for path, code := range map[string]int{"stations": 400, "stations?q=x&codec=FLAC": 400, "nope": 404} {
		if w := do(h, "GET", "/admin/api/directory/"+path, "", "test-token"); w.Code != code {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	srv.Close()
	if w := do(h, "GET", "/admin/api/directory/stations?country=DE", "", "test-token"); w.Code != http.StatusBadGateway {
		t.Fatalf("directory down: %d", w.Code)
	}
}

func TestBlankFolderNameRefused(t *testing.T) {
	h := setup(t)
	store, _ := h.Devices.Store(devices.DefaultKey)
	before := len(store.Snapshot().Folders)
	for _, name := range []string{"", "   ", `\t\n`, `\u00a0`, `\u200b`} {
		w := do(h, "PUT", "/admin/api/catalog", `{"stations":[],"folders":[{"name":"`+name+`"}]}`, "test-token")
		if w.Code != 400 || !strings.Contains(w.Body.String(), "name is empty") {
			t.Errorf("%q: %d %s", name, w.Code, w.Body.String())
		}
	}
	if len(store.Snapshot().Folders) != before {
		t.Fatal("a refused save changed the list")
	}
	if w := do(h, "PUT", "/admin/api/catalog", `{"stations":[],"folders":[{"name":"Empty but named"}]}`, "test-token"); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestPreview(t *testing.T) {
	h := setup(t)
	const filter = `{"country":"pl","language":"Polish","minBitrate":128,"order":"votes","limit":1}`
	if w := do(h, "POST", "/admin/api/directory/preview", filter, "test-token"); w.Code != 404 {
		t.Fatalf("directory off: %d", w.Code)
	}
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`[{"stationuuid":"u1","name":"Example One","url_resolved":"http://stream.example.com/1","codec":"MP3"},
		 {"stationuuid":"u2","name":"Example Two","url_resolved":"http://stream.example.com/2","codec":"MP3"}]`))
	}))
	h.Directory = directory.New(srv.URL)
	h.DirLimit = 100
	w := do(h, "POST", "/admin/api/directory/preview", filter, "test-token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Example One") || strings.Contains(w.Body.String(), "Example Two") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"countrycode=PL", "language=polish", "bitrateMin=128", "order=votes"} {
		if !strings.Contains(query, want) {
			t.Errorf("%q has no %s", query, want)
		}
	}
	for body, field := range map[string]string{`{}`: "filter", `{"order":"random"}`: "order", `{"limit":-1}`: "limit"} {
		if w := do(h, "POST", "/admin/api/directory/preview", body, "test-token"); w.Code != 400 || !strings.Contains(w.Body.String(), field) {
			t.Errorf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := do(h, "POST", "/admin/api/directory/preview", `{"nonsense":1}`, "test-token"); w.Code != 400 {
		t.Errorf("unknown field: %d", w.Code)
	}
	srv.Close()
	if w := do(h, "POST", "/admin/api/directory/preview", `{"country":"DE"}`, "test-token"); w.Code != http.StatusBadGateway {
		t.Fatalf("directory down: %d", w.Code)
	}
}

func TestVirtualFoldersInExportAndImport(t *testing.T) {
	h := setup(t)
	const list = `{"stations":[],"folders":[{"name":"Parent","folders":[{"name":"Polish talk","filter":{"country":"PL","language":"polish","minBitrate":128,"order":"votes","limit":20}}]}]}`
	if w := do(h, "PUT", "/admin/api/catalog", list, "test-token"); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	exported := do(h, "GET", "/admin/api/export", "", "test-token").Body.String()
	for _, want := range []string{`"language": "polish"`, `"minBitrate": 128`, `"order": "votes"`, `"limit": 20`} {
		if !strings.Contains(exported, want) {
			t.Errorf("export has no %s:\n%s", want, exported)
		}
	}
	if w := do(h, "POST", "/admin/api/import?mode=replace", exported, "test-token"); w.Code != 200 || !strings.Contains(w.Body.String(), `"limit": 20`) {
		t.Fatalf("replace: %d %s", w.Code, w.Body.String())
	}
	w := do(h, "POST", "/admin/api/import?mode=merge", exported, "test-token")
	if w.Code != 200 || strings.Count(w.Body.String(), `"limit": 20`) != 2 {
		t.Fatalf("merge: %d %s", w.Code, w.Body.String())
	}
}

// DSM's web server weakens ETags when it compresses an answer. The UI sends the version in
// its own header; If-Match still works, weak or strong.
func TestSaveVersionHeaders(t *testing.T) {
	h := setup(t)
	put := func(header, value, name string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "/admin/api/catalog", strings.NewReader(`{"stations":[{"name":"`+name+`","url":"http://stream.example.com/s"}],"folders":[]}`))
		r.Header.Set(TokenHeader, "test-token")
		r.Header.Set("Content-Type", "application/json")
		if header != "" {
			r.Header.Set(header, value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	version := func() string {
		w := do(h, "GET", "/admin/api/catalog", "", "test-token")
		v := w.Header().Get(VersionHeader)
		if v == "" || w.Header().Get("ETag") != `"`+v+`"` {
			t.Fatalf("version %q, ETag %q", v, w.Header().Get("ETag"))
		}
		return v
	}

	v := version()
	if w := put(VersionHeader, v, "a"); w.Code != 200 {
		t.Fatalf("version header: %d %s", w.Code, w.Body)
	}
	if w := put(VersionHeader, v, "b"); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale version header: %d", w.Code)
	}
	if w := put("If-Match", `W/"`+version()+`"`, "c"); w.Code != 200 {
		t.Fatalf("weak If-Match: %d %s", w.Code, w.Body)
	}
	// No version at all is an explicit overwrite.
	if w := put("", "", "d"); w.Code != 200 {
		t.Fatalf("overwrite: %d %s", w.Code, w.Body)
	}
	if got := do(h, "GET", "/admin/api/catalog", "", "test-token").Body.String(); !strings.Contains(got, `"d"`) {
		t.Fatal(got)
	}
}

func TestSecurityHeadersAndErrorStatus(t *testing.T) {
	h := setup(t)
	w := do(h, "GET", "/admin/", "", "test-token")
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers: %v", w.Header())
	}
	if w := do(h, "PUT", "/admin/api/catalog", `{"x":"`+strings.Repeat("a", maxBody)+`"}`, "test-token"); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too big: %d", w.Code)
	}
	if w := do(h, "GET", "/admin/api/catalogue", "", "test-token"); w.Code != http.StatusNotFound {
		t.Fatalf("/api/catalogue: %d", w.Code)
	}
	for err, want := range map[error]int{
		&fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}: http.StatusInternalServerError,
		errors.New("bad json"): http.StatusBadRequest,
	} {
		if got := errorStatus(err); got != want {
			t.Errorf("%v: got %d, want %d", err, got, want)
		}
	}
}
