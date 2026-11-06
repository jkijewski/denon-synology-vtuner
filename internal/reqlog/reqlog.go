// Package reqlog keeps the most recent receiver requests in memory.
package reqlog

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Entry is one logged request.
type Entry struct {
	Time   time.Time `json:"time"`
	Client string    `json:"client"`
	Host   string    `json:"host"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Query  string    `json:"query"`
	Status int       `json:"status"`
	Note   string    `json:"note,omitempty"`
}

// Log is a bounded ring of entries.
type Log struct {
	mu    sync.Mutex
	max   int
	items []Entry
}

// New returns a log that keeps at most max entries.
func New(max int) *Log {
	if max < 1 {
		max = 1
	}
	return &Log{max: max}
}

// Add appends e, dropping the oldest entry when full.
func (l *Log) Add(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.items) >= l.max {
		copy(l.items, l.items[1:])
		l.items = l.items[:len(l.items)-1]
	}
	l.items = append(l.items, e)
}

// Recent returns the entries, newest first.
func (l *Log) Recent() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Entry, len(l.items))
	for i, e := range l.items {
		out[len(l.items)-1-i] = e
	}
	return out
}

// ClientIP returns the original client address, trusting X-Forwarded-For / X-Real-IP
// only from the local reverse proxy.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host != "127.0.0.1" && host != "::1" {
		return host
	}
	forwarded, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ",")
	for _, v := range []string{r.Header.Get("X-Real-IP"), forwarded} {
		if v = strings.TrimSpace(v); net.ParseIP(v) != nil {
			return v
		}
	}
	return host
}
