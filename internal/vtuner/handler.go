// Package vtuner answers the HTTP requests a receiver sends to vTuner hosts.
package vtuner

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"denon-synology-vtuner/internal/catalog"
	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/directory"
	"denon-synology-vtuner/internal/relay"
	"denon-synology-vtuner/internal/reqlog"
)

// Handler serves the vTuner protocol.
type Handler struct {
	Devices       *devices.Registry
	Directory     *directory.Client // nil disables public directory browsing
	Log           *reqlog.Log
	DefaultHost   string // used in generated URLs when the request host is not a vTuner name
	MaxTitle      int    // titles longer than this (in characters) are shortened
	SearchEnabled bool
	Token         string // placeholder returned by the login handshake
	DirLimit      int    // max stations per directory listing
	TagCount      int    // number of genres offered
}

// IsVTunerHost reports whether host (with or without port) is vtuner.com or a sub-domain.
func IsVTunerHost(host string) bool {
	h := strings.ToLower(host)
	if i := strings.LastIndexByte(h, ':'); i >= 0 && !strings.Contains(h[i:], "]") {
		h = h[:i]
	}
	h = strings.TrimSuffix(h, ".")
	return h == "vtuner.com" || strings.HasSuffix(h, ".vtuner.com")
}

// query returns a case-insensitive view of the query: last non-empty value per lower-cased key.
func query(r *http.Request) map[string]string {
	out := map[string]string{}
	for k, vs := range r.URL.Query() {
		for _, v := range vs {
			if v != "" || out[strings.ToLower(k)] == "" {
				out[strings.ToLower(k)] = v
			}
		}
	}
	return out
}

type reqCtx struct {
	ctx    context.Context
	base   string // http://host
	prefix string // /setupapp/<vendor>/asp/BrowseXML/
	dev    string // receiver key
	store  *catalog.Store
}

func (c reqCtx) nav(params string) string {
	return fmt.Sprintf("%s%snavXML.asp?%s&dev=%s", c.base, c.prefix, params, c.dev)
}

func (h *Handler) newCtx(r *http.Request, q map[string]string) (reqCtx, error) {
	host := r.Host
	if !IsVTunerHost(host) {
		host = h.DefaultHost
	}
	vendor := "denon"
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) >= 2 && strings.EqualFold(parts[0], "setupapp") && parts[1] != "" {
		vendor = parts[1]
	}
	c := reqCtx{ctx: r.Context(), base: "http://" + host, prefix: "/setupapp/" + url.PathEscape(vendor) + "/asp/BrowseXML/"}
	ip := reqlog.ClientIP(r)
	var err error
	switch {
	case q["mac"] != "":
		c.dev, err = h.Devices.Login(q["mac"], ip)
	case devices.ValidKey(q["dev"]):
		c.dev = q["dev"]
	default:
		c.dev = h.Devices.ForIP(ip)
	}
	if err != nil {
		return c, err
	}
	c.store, err = h.Devices.Store(c.dev)
	if err != nil { // receiver deleted meanwhile
		c.dev = devices.DefaultKey
		c.store, err = h.Devices.Store(c.dev)
	}
	return c, err
}

// listed caps a directory station count at the number of stations a listing shows.
func (h *Handler) listed(n int) int {
	if h.DirLimit > 0 && n > h.DirLimit {
		return h.DirLimit
	}
	return n
}

func (h *Handler) short(s string) string {
	if h.MaxTitle <= 3 || utf8.RuneCountInString(s) <= h.MaxTitle {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:h.MaxTitle-3])) + "..."
}

func mimeOf(f string) string {
	f = strings.ToUpper(strings.TrimSpace(f))
	if f == "" {
		return "MP3"
	}
	return f
}

func (h *Handler) stationItem(c reqCtx, st catalog.Station) item {
	u := st.URL
	if st.Relay {
		u = fmt.Sprintf("%s/relay/%s/%d", c.base, c.dev, st.ID)
	}
	return item{kind: "Station", id: st.ID, name: h.short(st.Name), streamURL: u,
		desc: st.Description, logo: st.Logo, mime: mimeOf(st.Format), loc: st.Location, bitrate: st.Bitrate}
}

func (h *Handler) dirItem(c reqCtx, s directory.Station) item {
	u := s.StreamURL()
	if strings.HasPrefix(strings.ToLower(u), "https://") {
		u = c.base + "/relay/rb/" + url.PathEscape(s.UUID)
	}
	return item{kind: "Station", id: s.NumericID(), name: h.short(s.Name), streamURL: u,
		desc: s.Tags, logo: s.Favicon, mime: mimeOf(s.Codec), loc: s.Country, bitrate: s.Bitrate}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := query(r)
	op := strings.ToLower(path.Base(r.URL.Path))
	if op == "loginxml.asp" && q["token"] == "0" {
		h.write(w, r, []byte("<EncryptedToken>"+h.Token+"</EncryptedToken>"), "")
		return
	}
	c, err := h.newCtx(r, q)
	if err != nil {
		log.Printf("receiver lookup: %v", err)
		h.write(w, r, render(0, nil), "error: "+err.Error())
		return
	}
	note := ""
	var items []item

	switch op {
	case "loginxml.asp":
		items = h.root(c)
	case "navxml.asp":
		items, note = h.nav(c, q)
	case "statxml.asp":
		id, _ := strconv.Atoi(q["id"])
		if st, ok := c.store.Station(id); ok {
			items = []item{h.stationItem(c, st)}
		} else if h.Directory != nil {
			if s, ok := h.Directory.ByNumericID(id); ok {
				items = []item{h.dirItem(c, s)}
			}
		}
		if items == nil {
			note = "unknown station"
		}
	case "search.asp":
		text := q["search"]
		if text == "" {
			text = q["ssearch"]
		}
		// Search covers the receiver's own list only; the directory is searched in the admin UI.
		for _, st := range c.store.Search(text) {
			items = append(items, h.stationItem(c, st))
		}
	default:
		note = "unknown request"
	}

	items = named(items)
	total := len(items)
	items = page(items, q)
	h.write(w, r, render(total, items), note)
}

// named leaves out items a receiver would show as a row with no name. The catalog and the
// directory client already refuse such names, so this is the last line of defence.
func named(items []item) []item {
	keep := items[:0]
	for _, it := range items {
		if catalog.Blank(it.label()) {
			log.Printf("left a %s item without a name out of a menu", it.kind)
			continue
		}
		keep = append(keep, it)
	}
	return keep
}

// root is the top menu: the receiver's own folders, then directory browsing, then search when
// it's turned on. It is off by default because the Denon RCD-N7 shows the Search item as a row
// with no name that lists nothing.
func (h *Handler) root(c reqCtx) []item {
	items := h.folder(c, 0)
	if h.Directory != nil {
		countries, genres := h.directoryCounts(c)
		items = append(items,
			item{kind: "Dir", title: "Browse by country", urlDir: c.nav("gofile=countries"), dirCount: countries},
			item{kind: "Dir", title: "Browse by genre", urlDir: c.nav("gofile=genres"), dirCount: genres})
	}
	if len(items) == 0 {
		items = append(items, item{kind: "Display", text: "No stations yet. Add them in the vTuner Radio app in DSM."})
	}
	if h.SearchEnabled {
		items = append(items, item{kind: "Search", title: "Search",
			searchURL: fmt.Sprintf("%s%sSearch.asp?dev=%s&sSearchtype=2&search=", c.base, c.prefix, c.dev)})
	}
	return items
}

// directoryCounts returns how many countries and genres the browse menus hold.
// The lists are cached, so this is usually instant; when the directory is slow
// or unreachable the top menu is not held up and the counts are left at 0.
func (h *Handler) directoryCounts(c reqCtx) (countries, genres int) {
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()
	if list, err := h.Directory.Countries(ctx); err == nil {
		countries = len(list)
	}
	if list, err := h.Directory.Tags(ctx, h.TagCount); err == nil {
		genres = len(list)
	}
	return countries, genres
}

func (h *Handler) nav(c reqCtx, q map[string]string) ([]item, string) {
	switch q["gofile"] {
	case "folder":
		id, _ := strconv.Atoi(q["id"])
		if _, _, ok := c.store.List(id); !ok || id == 0 {
			return nil, "unknown folder"
		}
		if f := c.store.Filter(id); f != nil {
			return h.virtualFolder(c, *f)
		}
		return h.folder(c, id), ""
	case "countries", "genres", "country", "genre":
		if h.Directory == nil {
			return nil, "directory disabled"
		}
		items, err := h.browse(c, q)
		if err != nil {
			return []item{{kind: "Display", text: "Public directory not reachable. Try again later."}}, "directory: " + err.Error()
		}
		return items, ""
	}
	return h.root(c), ""
}

func (h *Handler) browse(c reqCtx, q map[string]string) ([]item, error) {
	var items []item
	switch q["gofile"] {
	case "countries":
		list, err := h.Directory.Countries(c.ctx)
		if err != nil {
			return nil, err
		}
		for _, x := range list {
			items = append(items, item{kind: "Dir", title: h.short(x.Name), dirCount: h.listed(x.StationCount),
				urlDir: c.nav("gofile=country&cc=" + url.QueryEscape(x.Code))})
		}
	case "genres":
		list, err := h.Directory.Tags(c.ctx, h.TagCount)
		if err != nil {
			return nil, err
		}
		for _, x := range list {
			items = append(items, item{kind: "Dir", title: h.short(x.Name), dirCount: h.listed(x.StationCount),
				urlDir: c.nav("gofile=genre&tag=" + url.QueryEscape(x.Name))})
		}
	case "country", "genre":
		dq := directory.Query{CountryCode: q["cc"], Tag: q["tag"], Limit: h.DirLimit}
		list, err := h.Directory.Stations(c.ctx, dq)
		if err != nil {
			return nil, err
		}
		for _, s := range list {
			items = append(items, h.dirItem(c, s))
		}
	}
	return items, nil
}

// folder lists folder id of the receiver's list (0 = top level).
func (h *Handler) folder(c reqCtx, id int) []item {
	entries, _, ok := c.store.List(id)
	if !ok {
		return nil
	}
	// All virtual folders of one listing share this limit, so a slow directory can't hold it up.
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()
	var items []item
	for _, e := range entries {
		if e.Folder != nil {
			count := e.ChildCount
			if e.Folder.Filter != nil {
				count = h.virtualCount(ctx, *e.Folder.Filter)
			}
			items = append(items, item{kind: "Dir", title: h.short(e.Folder.Name),
				urlDir: c.nav(fmt.Sprintf("gofile=folder&id=%d", e.Folder.ID)), dirCount: count})
		} else {
			items = append(items, h.stationItem(c, *e.Station))
		}
	}
	return items
}

// virtualCount returns how many stations a virtual folder lists, or 0 when the directory
// doesn't answer in time. Answers are cached, so opening the folder soon after is instant.
func (h *Handler) virtualCount(ctx context.Context, f catalog.Filter) int {
	if h.Directory == nil {
		return 0
	}
	list, err := h.Directory.Stations(ctx, directory.FromFilter(f, h.DirLimit))
	if err != nil {
		return 0
	}
	return len(list)
}

// virtualFolder lists the public directory stations that match f. Without a directory the list
// is empty and the reason goes to the request log.
func (h *Handler) virtualFolder(c reqCtx, f catalog.Filter) ([]item, string) {
	if h.Directory == nil {
		return nil, "directory disabled"
	}
	list, err := h.Directory.Stations(c.ctx, directory.FromFilter(f, h.DirLimit))
	if err != nil {
		return nil, "directory: " + err.Error()
	}
	items := make([]item, 0, len(list))
	for _, s := range list {
		items = append(items, h.dirItem(c, s))
	}
	return items, ""
}

// page applies startItems/endItems (1-based, inclusive).
func page(items []item, q map[string]string) []item {
	start, err1 := strconv.Atoi(q["startitems"])
	end, err2 := strconv.Atoi(q["enditems"])
	if err1 != nil && err2 != nil {
		return items
	}
	if err1 != nil || start < 1 {
		start = 1
	}
	if err2 != nil || end > len(items) {
		end = len(items)
	}
	if start > end {
		return nil
	}
	return items[start-1 : end]
}

func (h *Handler) write(w http.ResponseWriter, r *http.Request, body []byte, note string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
	if h.Log != nil {
		h.Log.Add(reqlog.Entry{Time: time.Now(), Client: reqlog.ClientIP(r), Host: r.Host, Method: r.Method,
			Path: r.URL.Path, Query: r.URL.RawQuery, Status: http.StatusOK, Note: note})
	}
}

var directoryUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// RelayResolver returns the target of /relay/<receiver>/<station id> and /relay/rb/<uuid>.
// Only stations in a receiver's list with the relay turned on, or public directory stations, are allowed.
// URLs that came from the public directory are marked, including those saved in a receiver's list.
func (h *Handler) RelayResolver(ctx context.Context, key string) (relay.Target, bool) {
	var none relay.Target
	a, b, ok := strings.Cut(key, "/")
	if !ok {
		return none, false
	}
	if a == "rb" {
		if h.Directory == nil || !directoryUUID.MatchString(b) {
			return none, false
		}
		s, err := h.Directory.ByUUID(ctx, b)
		if err != nil {
			return none, false
		}
		return relay.Target{URL: s.StreamURL(), Public: true}, true
	}
	if !devices.ValidKey(a) {
		return none, false
	}
	store, err := h.Devices.Store(a)
	if err != nil {
		return none, false
	}
	id, err := strconv.Atoi(b)
	if err != nil {
		return none, false
	}
	st, ok := store.Station(id)
	if !ok || !st.Relay {
		return none, false
	}
	return relay.Target{URL: st.URL, Public: st.Source != ""}, true
}
