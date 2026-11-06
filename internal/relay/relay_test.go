package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type station struct {
	ID    int
	URL   string
	Relay bool
}

// store returns a resolver over stations, keyed by "<id>", allowing only relay-enabled ones.
func store(t *testing.T, stations ...station) Resolver {
	return func(_ context.Context, key string) (Target, bool) {
		for _, s := range stations {
			if fmt.Sprint(s.ID) == key && s.Relay {
				return Target{URL: s.URL}, true
			}
		}
		return Target{}, false
	}
}

// local returns a relay that may connect to the test servers on the loopback address.
func local(r Resolver, max int) *Relay {
	rl := New(r, max)
	rl.Client = &http.Client{}
	return rl
}

func TestParsePlaylist(t *testing.T) {
	pls := "[playlist]\nNumberOfEntries=2\nFile1=http://a.example.com/1\nTitle1=x\nFile2=https://b.example.com/2\n"
	m3u := "#EXTM3U\n#EXTINF:-1,x\nhttp://c.example.com/3\n"
	if got := ParsePlaylist(pls); len(got) != 2 || got[1] != "https://b.example.com/2" {
		t.Fatal(got)
	}
	if got := ParsePlaylist(m3u); len(got) != 1 {
		t.Fatal(got)
	}
}

func TestRelayStreamsAndFollowsPlaylist(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/list.pls":
			fmt.Fprintf(w, "[playlist]\nFile1=http://%s/dead\nFile2=http://%s/live\n", r.Host, r.Host)
		case "/live":
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("AUDIO"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer up.Close()
	s := store(t, station{ID: 1002, URL: up.URL + "/list.pls", Relay: true},
		station{ID: 1003, URL: up.URL + "/live"})
	rl := local(s, 2)
	w := httptest.NewRecorder()
	rl.ServeHTTP(w, httptest.NewRequest("GET", "/relay/1002", nil))
	if w.Code != 200 || w.Body.String() != "AUDIO" || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	for _, p := range []string{"/relay/9999", "/relay/1003", "/relay/abc"} {
		w = httptest.NewRecorder()
		rl.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Fatalf("%s: want 404, got %d", p, w.Code)
		}
	}
}

func TestRelayLimitAndDisconnect(t *testing.T) {
	closed := make(chan struct{}, 4)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		for {
			if _, err := w.Write([]byte(strings.Repeat("x", 1024))); err != nil {
				closed <- struct{}{}
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				closed <- struct{}{}
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}))
	defer up.Close()
	s := store(t, station{ID: 1000, URL: up.URL + "/s", Relay: true})
	rl := local(s, 1)
	srv := httptest.NewServer(rl)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/relay/1000")
	if err != nil {
		t.Fatal(err)
	}
	io.ReadFull(resp.Body, make([]byte, 100))
	r2, _ := http.Get(srv.URL + "/relay/1000")
	if r2.StatusCode != 503 {
		t.Fatalf("want 503, got %d", r2.StatusCode)
	}
	r2.Body.Close()
	resp.Body.Close() // receiver stops playing
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream not closed after disconnect")
	}
}

func TestProbeSuggestsRelayForHTTPS(t *testing.T) {
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/aac")
		w.Write([]byte("x"))
	}))
	defer up.Close()
	rl := New(store(t), 1)
	rl.Client = up.Client()
	p := rl.Test(t.Context(), up.URL+"/live.aac")
	if !p.Reachable || !p.HTTPSOnly || !p.SuggestRelay {
		t.Fatalf("%+v", p)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProbeRedirectToHTTPSIsStillHTTPSOnly(t *testing.T) {
	rl := New(store(t), 1)
	rl.Client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "http" {
			u := *r.URL
			u.Scheme = "https"
			return &http.Response{StatusCode: 301, Header: http.Header{"Location": {u.String()}}, Body: http.NoBody, Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/mpeg"}}, Body: http.NoBody, Request: r}, nil
	})}
	if p := rl.Test(t.Context(), "https://stream.example.com/live.mp3"); !p.HTTPSOnly || !p.SuggestRelay {
		t.Fatalf("%+v", p)
	}
}

func TestProbeHTTPAlsoWorks(t *testing.T) {
	rl := New(store(t), 1)
	rl.Client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/mpeg"}}, Body: http.NoBody, Request: r}, nil
	})}
	if p := rl.Test(t.Context(), "https://stream.example.com/live.mp3"); p.HTTPSOnly {
		t.Fatalf("%+v", p)
	}
}

func TestAllowedAddresses(t *testing.T) {
	for _, c := range []struct {
		ip            string
		lan, internet bool
	}{
		{"127.0.0.1", false, false},
		{"::1", false, false},
		{"0.0.0.0", false, false},
		{"169.254.169.254", false, false},
		{"fe80::1", false, false},
		{"224.0.0.1", false, false},
		{"192.168.1.10", true, false},
		{"10.0.0.1", true, false},
		{"172.16.0.1", true, false},
		{"100.64.0.1", true, false},
		{"fd00::1", true, false},
		{"64:ff9b::a00:1", true, false},
		{"8.8.8.8", true, true},
		{"2606:4700:4700::1111", true, true},
	} {
		ip := netip.MustParseAddr(c.ip)
		if allowedLAN(ip) != c.lan || allowedInternet(ip) != c.internet {
			t.Errorf("%s: lan %v, internet %v", c.ip, allowedLAN(ip), allowedInternet(ip))
		}
	}
}

// The default clients must refuse a target on the NAS itself, however the relay gets there.
func TestRelayRefusesLoopback(t *testing.T) {
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("reached the loopback server: %s", r.URL)
	}))
	defer inner.Close()
	rl := New(func(_ context.Context, key string) (Target, bool) {
		return Target{URL: inner.URL + "/secret", Public: key == "public"}, true
	}, 1)
	for _, p := range []string{"/relay/public", "/relay/own"} {
		w := httptest.NewRecorder()
		rl.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != http.StatusBadGateway {
			t.Fatalf("%s: %d %q", p, w.Code, w.Body.String())
		}
	}
}

// Redirects and playlists from an allowed server must not lead to a refused address.
func TestRelayPublicTargetCantBeRedirectedInward(t *testing.T) {
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("reached the inner server: %s", r.URL)
	}))
	defer inner.Close()
	// The first hop goes through a fake transport, standing in for a server on the internet,
	// and the next hop uses the real dialer with its address check.
	real := newClient(allowedInternet).Transport
	fakeOutside := func(body string, redirect bool) *http.Client {
		return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "stream.example.com" {
				return real.RoundTrip(r)
			}
			if redirect {
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": {inner.URL + "/x"}}, Body: http.NoBody, Request: r}, nil
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/x-scpls"}},
				Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})}
	}
	for name, client := range map[string]*http.Client{
		"redirect": fakeOutside("", true),
		"playlist": fakeOutside("File1="+inner.URL+"/x\n", false),
	} {
		rl := New(func(context.Context, string) (Target, bool) {
			return Target{URL: "http://stream.example.com/s", Public: true}, true
		}, 1)
		rl.PublicClient = client
		w := httptest.NewRecorder()
		rl.ServeHTTP(w, httptest.NewRequest("GET", "/relay/x", nil))
		if w.Code != http.StatusBadGateway {
			t.Fatalf("%s: %d %q", name, w.Code, w.Body.String())
		}
	}
}

func TestRelayEndsStalledStream(t *testing.T) {
	release := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("AUDIO"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer up.Close()
	defer close(release)
	rl := local(store(t, station{ID: 1, URL: up.URL + "/s", Relay: true}), 1)
	rl.IdleTimeout = 50 * time.Millisecond
	done := make(chan struct{})
	w := httptest.NewRecorder()
	go func() {
		rl.ServeHTTP(w, httptest.NewRequest("GET", "/relay/1", nil))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stalled stream was not ended")
	}
	if len(rl.slots) != 0 {
		t.Fatal("slot not released")
	}
	if w.Body.String() != "AUDIO" {
		t.Fatalf("%q", w.Body.String())
	}
}
