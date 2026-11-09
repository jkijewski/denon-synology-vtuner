package reqlog

import (
	"net/http/httptest"
	"testing"
)

func TestRotation(t *testing.T) {
	l := New(3)
	for i := 1; i <= 4; i++ {
		l.Add(Entry{Status: i})
	}
	got := l.Recent()
	if len(got) != 3 || got[0].Status != 4 || got[2].Status != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestClientIP(t *testing.T) {
	for _, c := range []struct {
		name, remote, real, forwarded, want string
	}{
		{"direct client", "192.168.1.50:4000", "", "", "192.168.1.50"},
		{"headers ignored from other hosts", "192.168.1.50:4000", "192.168.1.99", "192.168.1.99", "192.168.1.50"},
		{"proxy with X-Real-IP", "127.0.0.1:4000", "192.168.1.50", "", "192.168.1.50"},
		{"proxy with both", "127.0.0.1:4000", "192.168.1.50", "192.168.1.50", "192.168.1.50"},
		{"proxy with X-Forwarded-For", "127.0.0.1:4000", "", "192.168.1.50", "192.168.1.50"},
		// The proxy appends the address it saw; a client can send any address before it.
		{"made-up X-Forwarded-For", "127.0.0.1:4000", "", "192.168.1.99, 192.168.1.50", "192.168.1.50"},
		{"made-up X-Real-IP behind a proxy that only appends", "127.0.0.1:4000", "192.168.1.99", "192.168.1.50", "192.168.1.50"},
		{"IPv6 proxy", "[::1]:4000", "192.168.1.50", "", "192.168.1.50"},
		{"proxy without headers", "127.0.0.1:4000", "", "", "127.0.0.1"},
		{"proxy with a header that isn't an address", "127.0.0.1:4000", "<script>", "", "127.0.0.1"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.real != "" {
			r.Header.Set("X-Real-IP", c.real)
		}
		if c.forwarded != "" {
			r.Header.Set("X-Forwarded-For", c.forwarded)
		}
		if got := ClientIP(r); got != c.want {
			t.Errorf("%s: %q", c.name, got)
		}
	}
}
