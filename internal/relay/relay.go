// Package relay passes a station's stream to the receiver over plain HTTP.
package relay

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"denon-synology-vtuner/internal/reqlog"
)

// Target is what a relay path points at.
type Target struct {
	URL string
	// Public marks a URL that came from the public directory. Anyone can add a station
	// there, so it may only reach addresses on the internet, never the NAS or the LAN.
	Public bool
}

// Resolver maps a relay path (the part after /relay/) to its target.
// It must only return URLs of known stations.
type Resolver func(ctx context.Context, path string) (Target, bool)

// DefaultIdleTimeout ends a stream when no data has moved for this long.
const DefaultIdleTimeout = 30 * time.Second

// DefaultPerClient is how many relayed streams one client address may hold at a time. A
// receiver plays one stream, and switching stations can overlap the old and new one briefly.
const DefaultPerClient = 2

// Relay serves /relay/....
type Relay struct {
	Resolve Resolver
	// Client is for stations in a receiver's list that didn't come from the public directory.
	// It reaches internet addresses only, unless AllowLAN was called. A list can be imported
	// from a file, so these URLs aren't trusted more than the directory by default.
	Client       *http.Client
	PublicClient *http.Client // for Target.Public: internet addresses only
	ProbeClient  *http.Client // for Test, which only an administrator can call: LAN allowed, the NAS itself not
	IdleTimeout  time.Duration
	PerClient    int // most streams per client address; 0 means DefaultPerClient
	slots        chan struct{}
	lan          bool // AllowLAN was called

	mu       sync.Mutex
	inFlight map[string]int // client address -> streams
}

// New returns a relay allowing max simultaneous streams, to internet addresses only.
func New(resolve Resolver, max int) *Relay {
	if max < 1 {
		max = 1
	}
	return &Relay{Resolve: resolve, slots: make(chan struct{}, max), IdleTimeout: DefaultIdleTimeout,
		Client: newClient(allowedInternet), PublicClient: newClient(allowedInternet), ProbeClient: newClient(allowedLAN),
		inFlight: map[string]int{}}
}

// AllowLAN lets stations that didn't come from the public directory reach addresses on the
// home network. The NAS's own addresses stay refused.
func (rl *Relay) AllowLAN() { rl.Client, rl.lan = newClient(allowedLAN), true }

// newClient returns a client that can only connect to addresses allowed reports true for.
// The check runs on the resolved address of every connection, so it also covers redirects,
// playlist entries and host names that resolve to a different address later. There is no
// proxy setting, because a proxy would make the connection on the client's behalf.
func newClient(allowed func(netip.Addr) bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return err
		}
		if ip := ap.Addr().Unmap(); !allowed(ip) {
			return &NotAllowedError{ip}
		}
		return nil
	}}
	return &http.Client{Transport: &http.Transport{
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: 15 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}}
}

// allowedLAN refuses the NAS itself, on its loopback and on its network addresses, and
// link-local addresses, which no stream server uses.
func allowedLAN(ip netip.Addr) bool {
	return !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() &&
		!thisHost.Contains(ip) && !ownAddr(ip)
}

// thisHost is "this network": a connection to 0.0.0.0 reaches the NAS itself on Linux.
var thisHost = netip.MustParsePrefix("0.0.0.0/8")

// ownAddr reports whether ip belongs to one of the NAS's network interfaces. DSM and other
// packages listen on these addresses, not only on the loopback address.
var ownAddr = func(ip netip.Addr) bool { return ownAddrs.has(ip) }

// ownAddrRefresh is how long a read of the interface addresses is used, as DHCP can change them.
const ownAddrRefresh = 30 * time.Second

type addrCache struct {
	mu    sync.Mutex
	at    time.Time
	addrs map[netip.Addr]bool
}

var ownAddrs addrCache

func (c *addrCache) has(ip netip.Addr) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.addrs == nil || time.Since(c.at) > ownAddrRefresh {
		addrs := map[netip.Addr]bool{}
		list, err := net.InterfaceAddrs()
		for _, a := range list {
			if p, err := netip.ParsePrefix(a.String()); err == nil {
				addrs[p.Addr().Unmap()] = true
			}
		}
		if err != nil && c.addrs != nil {
			addrs = c.addrs // keep the last good list
		}
		c.addrs, c.at = addrs, time.Now()
	}
	return c.addrs[ip.Unmap()]
}

// ErrNotAllowed is the cause of a refused connection.
var ErrNotAllowed = errors.New("address not allowed for this station")

// NotAllowedError names the refused address.
type NotAllowedError struct{ Addr netip.Addr }

func (e *NotAllowedError) Error() string {
	return fmt.Sprintf("address %s not allowed for this station", e.Addr)
}
func (e *NotAllowedError) Unwrap() error { return ErrNotAllowed }

// Special-use ranges that netip's Is* methods don't cover.
var nonInternet = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // can translate to private IPv4 addresses
	netip.MustParsePrefix("2001:db8::/32"),
}

func allowedInternet(ip netip.Addr) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || !allowedLAN(ip) {
		return false
	}
	for _, p := range nonInternet {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

const maxPlaylist = 64 << 10

// IsPlaylist reports whether a URL or content type names an .m3u or .pls playlist (not HLS).
func IsPlaylist(rawURL, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "scpls") || strings.Contains(ct, "mpegurl") && !strings.Contains(ct, "vnd.apple") {
		return true
	}
	p := strings.ToLower(rawURL)
	if u, err := url.Parse(rawURL); err == nil {
		p = strings.ToLower(u.Path)
	}
	return strings.HasSuffix(p, ".pls") || strings.HasSuffix(p, ".m3u")
}

// ParsePlaylist returns the stream URLs in an .m3u or .pls body, in order.
func ParsePlaylist(body string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '='); i > 0 && strings.HasPrefix(strings.ToLower(line), "file") {
			line = strings.TrimSpace(line[i+1:])
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			out = append(out, line)
		}
	}
	return out
}

// open fetches rawURL, following redirects and up to depth levels of playlists.
func (rl *Relay) open(ctx context.Context, client *http.Client, rawURL string, depth int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "denon-synology-vtuner/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	if !IsPlaylist(rawURL, resp.Header.Get("Content-Type")) {
		return resp, nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxPlaylist))
	resp.Body.Close()
	if depth <= 0 {
		return nil, errors.New("playlist nesting too deep")
	}
	lastErr := errors.New("playlist has no stream URLs")
	for _, u := range ParsePlaylist(string(b)) {
		r, err := rl.open(ctx, client, u, depth-1)
		if err == nil {
			return r, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// ServeHTTP implements http.Handler for paths under /relay/.
func (rl *Relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.Trim(strings.TrimPrefix(r.URL.Path, "/relay/"), "/")
	target, ok := rl.Resolve(r.Context(), key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// The per-client limit comes first, so that one client can't take every slot.
	who := reqlog.ClientIP(r)
	if !rl.takeClientSlot(who) {
		http.Error(w, "too many relayed streams for this client", http.StatusServiceUnavailable)
		return
	}
	defer rl.releaseClientSlot(who)
	select {
	case rl.slots <- struct{}{}:
		defer func() { <-rl.slots }()
	default:
		http.Error(w, "too many relayed streams", http.StatusServiceUnavailable)
		return
	}
	client := rl.Client
	if target.Public {
		client = rl.PublicClient
	}
	// The context ends when the receiver disconnects or the stream stalls, which closes the upstream connection.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	resp, err := rl.open(ctx, client, target.URL, 3)
	if err != nil {
		hint := ""
		var refused *NotAllowedError
		if !target.Public && !rl.lan && errors.As(err, &refused) && allowedLAN(refused.Addr) {
			hint = "; if this station is on your home network, set VTUNER_RELAY_LAN=true"
		}
		log.Printf("relay %q: %v%s", key, err, hint)
		http.Error(w, "stream not reachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", audioType(resp.Header.Get("Content-Type")))
	w.Header().Set("Cache-Control", "no-cache")
	// The answer comes from someone else's server and is shown under a vtuner.com name on the
	// NAS, so a browser must never treat it as a page.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	idle := rl.IdleTimeout
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}
	stalled := time.AfterFunc(idle, cancel)
	defer stalled.Stop()
	rc := http.NewResponseController(w)
	buf := make([]byte, 16<<10)
	for {
		stalled.Reset(idle)
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			// Errors mean the writer has no deadlines or flushing, as in tests; a real
			// failure shows up in the Write.
			_ = rc.SetWriteDeadline(time.Now().Add(idle))
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			_ = rc.Flush()
		}
		if rerr != nil {
			return
		}
	}
}

// audioType returns the upstream content type if it is one a stream uses, else audio/mpeg.
// Any other type, such as text/html, could make a browser show the answer as a page.
func audioType(ct string) string {
	mt, _, err := mime.ParseMediaType(ct)
	if err == nil && (strings.HasPrefix(mt, "audio/") || mt == "application/ogg" || mt == "application/octet-stream") {
		return ct
	}
	return "audio/mpeg"
}

func (rl *Relay) takeClientSlot(who string) bool {
	limit := rl.PerClient
	if limit <= 0 {
		limit = DefaultPerClient
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.inFlight == nil {
		rl.inFlight = map[string]int{}
	}
	if rl.inFlight[who] >= limit {
		return false
	}
	rl.inFlight[who]++
	return true
}

func (rl *Relay) releaseClientSlot(who string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.inFlight[who]--; rl.inFlight[who] <= 0 {
		delete(rl.inFlight, who)
	}
}

// Probe is the result of testing a station URL.
type Probe struct {
	Reachable    bool   `json:"reachable"`
	Status       int    `json:"status,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	FinalURL     string `json:"finalUrl,omitempty"`
	HTTPSOnly    bool   `json:"httpsOnly"`
	SuggestRelay bool   `json:"suggestRelay"`
	Warning      string `json:"warning,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Test checks whether rawURL plays and whether the relay is needed.
func (rl *Relay) Test(ctx context.Context, rawURL string) Probe {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	p := Probe{}
	resp, err := rl.open(ctx, rl.ProbeClient, rawURL, 3)
	if err != nil {
		p.Error = err.Error()
		return p
	}
	resp.Body.Close()
	p.Reachable, p.Status = true, resp.StatusCode
	p.ContentType = resp.Header.Get("Content-Type")
	p.FinalURL = resp.Request.URL.String()
	ct := strings.ToLower(p.ContentType)
	switch {
	case strings.Contains(ct, "vnd.apple.mpegurl") || strings.HasSuffix(strings.ToLower(resp.Request.URL.Path), ".m3u8"):
		p.Warning = "HLS stream: the receiver can't play it and the relay doesn't convert it. Pick another stream URL."
	case ct != "" && !strings.HasPrefix(ct, "audio/") && !strings.Contains(ct, "ogg") && !strings.Contains(ct, "octet-stream"):
		p.Warning = "Content type is not audio. Check that this is a direct stream URL."
	case strings.Contains(ct, "aac") || strings.Contains(ct, "ogg") || strings.Contains(ct, "flac"):
		p.Warning = "Not MP3. Older receivers may not play this format."
	}
	if resp.Request.URL.Scheme == "https" {
		p.HTTPSOnly = true
		if u, err := url.Parse(p.FinalURL); err == nil {
			u.Scheme = "http"
			if r2, err := rl.open(ctx, rl.ProbeClient, u.String(), 1); err == nil {
				// A redirect back to HTTPS doesn't count as plain HTTP working.
				p.HTTPSOnly = r2.Request.URL.Scheme == "https"
				r2.Body.Close()
			}
		}
		p.SuggestRelay = true
	}
	if IsPlaylist(rawURL, "") {
		p.SuggestRelay = true
	}
	return p
}
