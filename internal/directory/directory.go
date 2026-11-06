// Package directory reads the public radio-browser.info station directory.
package directory

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"denon-synology-vtuner/internal/catalog"
)

// Station is a public directory entry, reduced to what the receiver and admin UI need.
type Station struct {
	UUID        string `json:"stationuuid"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	URLResolved string `json:"url_resolved"`
	Homepage    string `json:"homepage"`
	Favicon     string `json:"favicon"`
	Tags        string `json:"tags"`
	Country     string `json:"country"`
	CountryCode string `json:"countrycode"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	HLS         int    `json:"hls"`
	LastCheckOK int    `json:"lastcheckok"`
	ClickCount  int    `json:"clickcount"`
	Votes       int    `json:"votes"`
	ClickTrend  int    `json:"clicktrend"`
}

// StreamURL returns the best URL to play.
func (s Station) StreamURL() string {
	if s.URLResolved != "" {
		return s.URLResolved
	}
	return s.URL
}

// NumericID is a stable number for the station, for receivers that expect numeric ids.
func (s Station) NumericID() int {
	return int(crc32.ChecksumIEEE([]byte(s.UUID))&0x3fffffff) + 1<<30
}

// Country is one country with its station count.
type Country struct {
	Name         string `json:"name"`
	Code         string `json:"iso_3166_1"`
	StationCount int    `json:"stationcount"`
}

// Tag is one genre tag with its station count.
type Tag struct {
	Name         string `json:"name"`
	StationCount int    `json:"stationcount"`
}

// Query selects stations.
type Query struct {
	CountryCode string
	Tag         string
	AnyTag      []string // stations with any of these genres; takes the place of Tag
	Name        string
	Codec       string // one of the client's Codecs, e.g. "MP3"; empty means any playable codec
	Language    string
	MinBitrate  int    // kbit/s
	Order       string // the directory's name for a sort order; empty means most played first
	Limit       int    // most stations to return; 0 means 200
}

// FromFilter is the query of a virtual folder. max is the most stations a listing shows.
func FromFilter(f catalog.Filter, max int) Query {
	limit := max
	if f.Limit > 0 && (max <= 0 || f.Limit < max) {
		limit = f.Limit
	}
	return Query{CountryCode: f.Country, AnyTag: f.Genres, Name: f.Name, Codec: f.Format, Language: f.Language,
		MinBitrate: f.MinBitrate, Order: f.Order, Limit: limit}
}

// DiscoverHost is the DNS name whose addresses are the radio-browser API servers.
const DiscoverHost = "all.api.radio-browser.info"

// FallbackServer is used when discovery finds nothing.
const FallbackServer = "https://de1.api.radio-browser.info"

const (
	discoverEvery = time.Hour   // how long a discovered server list is used
	discoverRetry = time.Minute // wait after a discovery that found nothing
)

// Discover returns the API servers as https base URLs, as the radio-browser docs recommend:
// look up all.api.radio-browser.info and reverse-resolve each address to its server name.
// The result is empty when nothing is found.
func Discover(ctx context.Context) []string {
	var r net.Resolver
	ips, _ := r.LookupHost(ctx, DiscoverHost)
	seen := map[string]bool{}
	var out []string
	for _, ip := range ips {
		names, _ := r.LookupAddr(ctx, ip)
		for _, n := range names {
			n = strings.TrimSuffix(n, ".")
			if strings.HasSuffix(n, ".api.radio-browser.info") && !seen[n] {
				seen[n] = true
				out = append(out, "https://"+n)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Client calls the radio-browser API with caching.
type Client struct {
	// Base is the API base URL, e.g. https://de1.api.radio-browser.info. When empty,
	// servers are discovered with Discover and the next one is tried on failure.
	Base      string
	Codecs    []string // allowed codecs (upper case); empty allows all
	UserAgent string
	HTTP      *http.Client
	TTL       time.Duration

	discover func(context.Context) []string

	mu           sync.Mutex
	cache        map[string]cached
	ids          map[int]Station // numeric id -> station, filled by listings
	servers      []string
	nextDiscover time.Time
}

type cached struct {
	at   time.Time
	body []byte
}

// New returns a client for base ("" discovers the servers).
func New(base string) *Client {
	return &Client{Base: strings.TrimRight(base, "/"), Codecs: []string{"MP3", "AAC", "AAC+"},
		UserAgent: "denon-synology-vtuner/1.0", HTTP: &http.Client{Timeout: 10 * time.Second}, TTL: 30 * time.Minute,
		discover: Discover, cache: map[string]cached{}, ids: map[int]Station{}}
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	c.mu.Lock()
	if e, ok := c.cache[path]; ok && time.Since(e.at) < c.TTL {
		c.mu.Unlock()
		return json.Unmarshal(e.body, v)
	}
	c.mu.Unlock()
	var raw json.RawMessage
	var err error
	for _, base := range c.bases(ctx) {
		if raw, err = c.fetch(ctx, base+path); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	c.mu.Lock()
	now := time.Now()
	for k, e := range c.cache {
		if now.Sub(e.at) >= c.TTL {
			delete(c.cache, k)
		}
	}
	if len(c.cache) > 500 {
		c.cache = map[string]cached{}
	}
	c.cache[path] = cached{now, raw}
	c.mu.Unlock()
	return json.Unmarshal(raw, v)
}

func (c *Client) bases(ctx context.Context) []string {
	if c.Base != "" {
		return []string{c.Base}
	}
	now := time.Now()
	c.mu.Lock()
	known := c.servers
	due := !now.Before(c.nextDiscover)
	if due {
		c.nextDiscover = now.Add(discoverRetry) // other callers don't wait for this lookup
	}
	c.mu.Unlock()
	if due {
		// DNS can be slow, so it runs without the lock that cached answers need.
		found := c.discover(ctx)
		c.mu.Lock()
		switch {
		case len(found) > 0:
			c.servers, known = found, found
			c.nextDiscover = now.Add(discoverEvery)
		case ctx.Err() != nil:
			c.nextDiscover = time.Time{} // this caller gave up, which says nothing about DNS
		}
		c.mu.Unlock()
	}
	if len(known) == 0 {
		return []string{FallbackServer} // for this call only: discovery is tried again later
	}
	return known
}

func (c *Client) fetch(ctx context.Context, u string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("directory: status %d", resp.StatusCode)
	}
	var raw json.RawMessage
	err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&raw)
	return raw, err
}

// Countries returns countries with stations, sorted by name. Entries without a
// name are dropped, entries for the same country are merged, and long official
// names are replaced by their common short form.
func (c *Client) Countries(ctx context.Context) ([]Country, error) {
	var out []Country
	if err := c.get(ctx, "/json/countries?hidebroken=true", &out); err != nil {
		return nil, err
	}
	return cleanCountries(out), nil
}

func cleanCountries(in []Country) []Country {
	byCode := map[string]int{} // code -> index in keep
	byName := map[string]int{} // lower-case display name -> index in keep
	var keep []Country
	for _, x := range in {
		x.Code = strings.ToUpper(strings.TrimSpace(x.Code))
		x.Name = countryName(x.Code, x.Name)
		if x.StationCount <= 0 || x.Code == "" || x.Name == "" {
			continue
		}
		if i, ok := byCode[x.Code]; ok {
			keep[i].StationCount += x.StationCount
			continue
		}
		key := strings.ToLower(x.Name)
		if i, ok := byName[key]; ok {
			// Same name under another code: keep the code with more stations.
			if x.StationCount > keep[i].StationCount {
				byCode[x.Code] = i
				keep[i] = x
			}
			continue
		}
		byCode[x.Code] = len(keep)
		byName[key] = len(keep)
		keep = append(keep, x)
	}
	sort.Slice(keep, func(i, j int) bool { return strings.ToLower(keep[i].Name) < strings.ToLower(keep[j].Name) })
	return keep
}

// shortCountryNames replaces long official names that don't fit a receiver's display.
var shortCountryNames = map[string]string{
	"BO": "Bolivia", "BQ": "Caribbean Netherlands", "CD": "DR Congo", "FM": "Micronesia",
	"GB": "United Kingdom", "GS": "South Georgia", "HM": "Heard and McDonald Islands",
	"IO": "British Indian Ocean Territory", "IR": "Iran", "KP": "North Korea", "KR": "South Korea",
	"LA": "Laos", "MD": "Moldova", "MK": "North Macedonia", "PS": "Palestine", "RU": "Russia",
	"SH": "Saint Helena", "SY": "Syria", "TW": "Taiwan", "TZ": "Tanzania", "UM": "US Outlying Islands",
	"US": "United States", "VE": "Venezuela", "VG": "British Virgin Islands", "VI": "US Virgin Islands",
	"VN": "Vietnam",
}

func countryName(code, name string) string {
	if n, ok := shortCountryNames[code]; ok {
		return n
	}
	name = strings.Join(strings.Fields(catalog.CleanName(name)), " ")
	if i := strings.Index(name, " ("); i > 0 { // "Bolivia (Plurinational State Of)"
		name = name[:i]
	}
	return strings.TrimPrefix(name, "The ")
}

// Tags returns the n most used genre tags, sorted by name. Tags without a name are dropped.
func (c *Client) Tags(ctx context.Context, n int) ([]Tag, error) {
	return c.tagList(ctx, "tags", n)
}

// Languages returns the n most used languages, sorted by name. Entries without a name are dropped.
func (c *Client) Languages(ctx context.Context, n int) ([]Tag, error) {
	return c.tagList(ctx, "languages", n)
}

func (c *Client) tagList(ctx context.Context, kind string, n int) ([]Tag, error) {
	var out []Tag
	if err := c.get(ctx, fmt.Sprintf("/json/%s?hidebroken=true&order=stationcount&reverse=true&limit=%d", kind, n), &out); err != nil {
		return nil, err
	}
	keep := out[:0]
	for _, x := range out {
		if x.Name = catalog.CleanName(x.Name); x.Name != "" {
			keep = append(keep, x)
		}
	}
	sort.Slice(keep, func(i, j int) bool { return keep[i].Name < keep[j].Name })
	return keep, nil
}

// Stations returns working, non-HLS stations with a name that match q, most played first
// unless q says otherwise.
func (c *Client) Stations(ctx context.Context, q Query) ([]Station, error) {
	switch len(q.AnyTag) {
	case 0:
	case 1:
		q.Tag, q.AnyTag = q.AnyTag[0], nil
	default:
		return c.stationsAnyTag(ctx, q)
	}
	v := url.Values{}
	v.Set("hidebroken", "true")
	order := q.Order
	if order == "" {
		order = "clickcount"
	}
	v.Set("order", order)
	v.Set("reverse", fmt.Sprint(order != "name")) // best first, but names from A to Z
	if q.Limit <= 0 {
		q.Limit = 200
	}
	// Some answers are dropped below, so ask for more to still fill the listing.
	v.Set("limit", fmt.Sprint(q.Limit*2))
	if q.Language != "" {
		v.Set("language", q.Language)
	}
	if q.MinBitrate > 0 {
		v.Set("bitrateMin", fmt.Sprint(q.MinBitrate))
	}
	if q.CountryCode != "" {
		v.Set("countrycode", q.CountryCode)
	}
	if q.Tag != "" {
		v.Set("tag", q.Tag)
		v.Set("tagExact", "true")
	}
	if q.Name != "" {
		v.Set("name", q.Name)
	}
	codec := strings.ToUpper(strings.TrimSpace(q.Codec))
	if codec != "" {
		if !c.codecOK(codec) {
			return nil, fmt.Errorf("format %q is not playable by the receiver", q.Codec)
		}
		v.Set("codec", codec)
	}
	var out []Station
	if err := c.get(ctx, "/json/stations/search?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	keep := out[:0]
	for _, s := range out {
		if s.HLS == 1 || s.StreamURL() == "" || !c.codecOK(s.Codec) {
			continue
		}
		if codec != "" && strings.ToUpper(strings.TrimSpace(s.Codec)) != codec {
			continue
		}
		if s.Name = catalog.CleanName(s.Name); s.Name == "" {
			continue // a row without a name is no use on a receiver
		}
		keep = append(keep, s)
		if len(keep) == q.Limit {
			break
		}
	}
	c.remember(keep)
	return keep, nil
}

// stationsAnyTag merges one search per genre, because the directory's own list of tags only
// finds stations that have all of them. The searches run together, as the menu that counts a
// virtual folder's stations waits only 2 seconds. Each search returns the best q.Limit of its
// genre, so the merged list holds the best q.Limit overall.
func (c *Client) stationsAnyTag(ctx context.Context, q Query) ([]Station, error) {
	if q.Limit <= 0 {
		q.Limit = 200
	}
	lists := make([][]Station, len(q.AnyTag))
	errs := make([]error, len(q.AnyTag))
	var wg sync.WaitGroup
	for i, tag := range q.AnyTag {
		wg.Go(func() {
			one := q
			one.Tag, one.AnyTag = tag, nil
			lists[i], errs[i] = c.Stations(ctx, one)
		})
	}
	wg.Wait()
	seen := map[string]bool{}
	var out []Station
	for i, list := range lists {
		if errs[i] != nil {
			return nil, errs[i]
		}
		for _, s := range list {
			if !seen[s.UUID] {
				seen[s.UUID] = true
				out = append(out, s)
			}
		}
	}
	slices.SortFunc(out, func(a, b Station) int { return compareStations(q.Order, a, b) })
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// compareStations sorts like the directory does for order. Ties go by name and id, so that
// pages of a listing stay in step.
func compareStations(order string, a, b Station) int {
	byValue := 0
	switch order {
	case "name":
	case "votes":
		byValue = cmp.Compare(b.Votes, a.Votes)
	case "clicktrend":
		byValue = cmp.Compare(b.ClickTrend, a.ClickTrend)
	case "bitrate":
		byValue = cmp.Compare(b.Bitrate, a.Bitrate)
	default:
		byValue = cmp.Compare(b.ClickCount, a.ClickCount)
	}
	return cmp.Or(byValue, cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.UUID, b.UUID))
}

func (c *Client) codecOK(codec string) bool {
	if len(c.Codecs) == 0 {
		return true
	}
	codec = strings.ToUpper(strings.TrimSpace(codec))
	for _, k := range c.Codecs {
		if k == codec {
			return true
		}
	}
	return false
}

func (c *Client) remember(list []Station) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.ids) > 20000 {
		c.ids = map[int]Station{}
	}
	for _, s := range list {
		c.ids[s.NumericID()] = s
	}
}

// ByNumericID returns a station seen in an earlier listing.
func (c *Client) ByNumericID(id int) (Station, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.ids[id]
	return s, ok
}

// ByUUID looks a station up in the directory.
func (c *Client) ByUUID(ctx context.Context, uuid string) (Station, error) {
	var out []Station
	if err := c.get(ctx, "/json/stations/byuuid/"+url.PathEscape(uuid), &out); err != nil {
		return Station{}, err
	}
	if len(out) == 0 {
		return Station{}, fmt.Errorf("station %s not in directory", uuid)
	}
	c.remember(out)
	return out[0], nil
}
