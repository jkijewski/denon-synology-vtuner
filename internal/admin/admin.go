// Package admin serves the admin UI and its JSON API.
package admin

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"slices"
	"strings"

	"denon-synology-vtuner/internal/catalog"
	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/directory"
	"denon-synology-vtuner/internal/relay"
	"denon-synology-vtuner/internal/reqlog"
)

//go:embed ui
var uiFS embed.FS

// TokenHeader carries the shared secret between the DSM CGI and the server.
const TokenHeader = "X-Vtuner-Admin-Token"

// Handler serves /admin/.
type Handler struct {
	Devices    *devices.Registry
	Directory  *directory.Client // nil when the public directory is turned off
	DirLimit   int               // max stations per directory listing on the receiver
	Log        *reqlog.Log
	Relay      *relay.Relay
	Token      string // required in TokenHeader; empty disables the check (dev mode only)
	ServiceLog string // path of the service log file
	Version    string
}

const maxBody = 4 << 20

// contentSecurityPolicy lets the page load only its own files. The data: images are the
// select arrows in style.css. DSM may show the page in a frame of its own site.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'self'"

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Token != "" {
		got := r.Header.Get(TokenHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(h.Token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// A web page on another site can't send this content type without the browser asking first.
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "content type must be application/json"})
			return
		}
	}
	p := strings.TrimPrefix(r.URL.Path, "/admin")
	switch {
	case p == "/api/catalog" || p == "/api/export" || p == "/api/import":
		h.catalogAPI(w, r, p)
	case strings.HasPrefix(p, "/api/"):
		h.api(w, r, p)
	default:
		h.static(w, r, p)
	}
}

// catalogAPI serves the calls on one receiver's list, named by the device parameter.
func (h *Handler) catalogAPI(w http.ResponseWriter, r *http.Request, p string) {
	key := r.URL.Query().Get("device")
	if key == "" {
		key = devices.DefaultKey
	}
	store, err := h.store(key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown receiver"})
		return
	}
	switch {
	case p == "/api/catalog" && r.Method == http.MethodGet:
		writeCatalog(w, store)
	case p == "/api/catalog" && r.Method == http.MethodPut:
		h.replace(w, r, store)
	case p == "/api/export" && r.Method == http.MethodGet:
		w.Header().Set("Content-Disposition", `attachment; filename="vtuner-`+key+`.json"`)
		writeCatalog(w, store)
	case p == "/api/import" && r.Method == http.MethodPost:
		h.importCatalog(w, r, store)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// api serves the other calls.
func (h *Handler) api(w http.ResponseWriter, r *http.Request, p string) {
	switch {
	case p == "/api/devices" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.Devices.List())
	case strings.HasPrefix(p, "/api/devices/"):
		h.device(w, r, strings.TrimPrefix(p, "/api/devices/"))
	case p == "/api/directory/preview" && r.Method == http.MethodPost:
		h.preview(w, r)
	case strings.HasPrefix(p, "/api/directory/") && r.Method == http.MethodGet:
		h.directory(w, r, strings.TrimPrefix(p, "/api/directory/"))
	case p == "/api/test-url" && r.Method == http.MethodPost:
		h.testURL(w, r)
	case p == "/api/requests" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.Log.Recent())
	case p == "/api/log" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(tail(h.ServiceLog, 64<<10))
	case p == "/api/info" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"version": h.Version})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *Handler) testURL(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in); err != nil || in.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url missing"})
		return
	}
	writeJSON(w, http.StatusOK, h.Relay.Test(r.Context(), in.URL))
}

func (h *Handler) static(w http.ResponseWriter, r *http.Request, p string) {
	if p == "" || p == "/" {
		p = "/index.html"
	}
	sub, _ := fs.Sub(uiFS, "ui")
	b, err := fs.ReadFile(sub, strings.TrimPrefix(p, "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(p, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(p, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(p, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	w.Write(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// VersionHeader carries the catalog version. A PUT that gives it only succeeds if the
// catalog is still at that version. ETag carries the same value, but DSM's web server turns it
// into a weak tag ("W/...") when it compresses the answer, so the UI uses this header instead.
const VersionHeader = "X-Catalog-Version"

// writeCatalog sends the catalog with the version that a later PUT can give back.
func writeCatalog(w http.ResponseWriter, store *catalog.Store) {
	d, etag := store.SnapshotETag()
	w.Header().Set(VersionHeader, etag)
	w.Header().Set("ETag", `"`+etag+`"`)
	writeJSON(w, http.StatusOK, d)
}

// expectedVersion returns the catalog version a PUT was based on, or "" for no check
// (an explicit overwrite). It reads VersionHeader, else If-Match, strong or weak.
func expectedVersion(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get(VersionHeader)); v != "" {
		return v
	}
	v := strings.TrimSpace(r.Header.Get("If-Match"))
	v = strings.TrimPrefix(v, "W/")
	return strings.Trim(v, `"`)
}

func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, catalog.ErrConflict) {
		writeJSON(w, http.StatusPreconditionFailed, map[string]string{"error": err.Error()})
		return
	}
	var fe *catalog.FieldError
	if errors.As(err, &fe) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fe.Error(), "field": fe})
		return
	}
	writeJSON(w, errorStatus(err), map[string]string{"error": err.Error()})
}

// errorStatus tells a request that was too big, and a failure on the NAS, from a bad request.
func errorStatus(err error) int {
	var tooBig *http.MaxBytesError
	var path *fs.PathError
	var link *os.LinkError
	switch {
	case errors.As(err, &tooBig):
		return http.StatusRequestEntityTooLarge
	case errors.As(err, &path), errors.As(err, &link):
		return http.StatusInternalServerError
	}
	return http.StatusBadRequest
}

func (h *Handler) store(key string) (*catalog.Store, error) {
	if !devices.ValidKey(key) {
		return nil, os.ErrNotExist
	}
	return h.Devices.Store(key)
}

// device handles PATCH (rename), DELETE, and POST .../copy?from=<key>.
func (h *Handler) device(w http.ResponseWriter, r *http.Request, rest string) {
	key, action, _ := strings.Cut(rest, "/")
	store, err := h.store(key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown receiver"})
		return
	}
	switch {
	case action == "" && r.Method == http.MethodPatch:
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
			writeErr(w, err)
			return
		}
		if err := h.Devices.Rename(key, in.Name); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, h.Devices.List())
	case action == "" && r.Method == http.MethodDelete:
		if err := h.Devices.Delete(key); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, h.Devices.List())
	case action == "copy" && r.Method == http.MethodPost:
		src, err := h.store(r.URL.Query().Get("from"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown source receiver"})
			return
		}
		if err := store.Replace(src.Snapshot()); err != nil {
			writeErr(w, err)
			return
		}
		writeCatalog(w, store)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// preview answers with the stations a virtual folder with the posted filter lists on the receiver.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	if h.Directory == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "public directory is turned off"})
		return
	}
	var f catalog.Filter
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		writeErr(w, err)
		return
	}
	if err := f.Normalize("preview"); err != nil {
		writeErr(w, err)
		return
	}
	list, err := h.Directory.Stations(r.Context(), directory.FromFilter(f, h.DirLimit))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "public directory not reachable: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// directory proxies the public directory: countries, tags, languages, stations?country=&tag=&q=&codec=.
func (h *Handler) directory(w http.ResponseWriter, r *http.Request, what string) {
	if h.Directory == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "public directory is turned off"})
		return
	}
	var v any
	var err error
	switch what {
	case "countries":
		v, err = h.Directory.Countries(r.Context())
	case "tags":
		v, err = h.Directory.Tags(r.Context(), 300)
	case "languages":
		v, err = h.Directory.Languages(r.Context(), 100)
	case "stations":
		q := r.URL.Query()
		dq := directory.Query{CountryCode: q.Get("country"), Tag: q.Get("tag"), Name: q.Get("q"), Codec: q.Get("codec"), Limit: 300}
		if dq.CountryCode == "" && dq.Tag == "" && dq.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "choose a country, a genre or a search text"})
			return
		}
		if dq.Codec != "" && !slices.Contains(h.Directory.Codecs, strings.ToUpper(dq.Codec)) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown format " + dq.Codec})
			return
		}
		v, err = h.Directory.Stations(r.Context(), dq)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "public directory not reachable: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request, store *catalog.Store) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeErr(w, err)
		return
	}
	d, err := catalog.Parse(b)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := store.ReplaceIf(expectedVersion(r), d); err != nil {
		writeErr(w, err)
		return
	}
	writeCatalog(w, store)
}

func (h *Handler) importCatalog(w http.ResponseWriter, r *http.Request, store *catalog.Store) {
	mode := r.URL.Query().Get("mode")
	if mode != "merge" && mode != "replace" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be merge or replace"})
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeErr(w, err)
		return
	}
	d, err := catalog.Parse(b)
	if err != nil {
		writeErr(w, err)
		return
	}
	if mode == "merge" {
		err = store.Merge(d)
	} else {
		err = store.Replace(d)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeCatalog(w, store)
}

func tail(path string, n int64) []byte {
	f, err := os.Open(path)
	if err != nil {
		return []byte("(no log yet)\n")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	if st.Size() > n {
		f.Seek(st.Size()-n, io.SeekStart)
	}
	b, _ := io.ReadAll(f)
	return b
}
