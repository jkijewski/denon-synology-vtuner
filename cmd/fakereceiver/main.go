// Command fakereceiver walks a vTuner server's menus the way a receiver does, for testing
// without real hardware.
//
//	fakereceiver -server 127.0.0.1:80            interactive: pick menu entries by number
//	fakereceiver -server 127.0.0.1:80 -auto      smoke test: walk the menus and play one station
//
// Requests use the real vtuner.com URLs and are sent to -server, the same way a receiver
// whose DNS points at the server would send them.
package main

import (
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"denon-synology-vtuner/internal/catalog"
)

type item struct {
	Type      string `xml:"ItemType"`
	Title     string `xml:"Title"`
	URLDir    string `xml:"UrlDir"`
	Count     int    `xml:"DirCount"`
	ID        string `xml:"StationId"`
	Name      string `xml:"StationName"`
	StreamURL string `xml:"StationUrl"`
	Mime      string `xml:"StationMime"`
	SearchURL string `xml:"SearchURL"`
	Display   string `xml:"Display"`
}

type list struct {
	Count int    `xml:"ItemCount"`
	Items []item `xml:"Item"`
}

// checkNames fails when a receiver would show a row with no name.
func checkNames(menu string, items []item) error {
	for i, it := range items {
		text := it.Title
		switch it.Type {
		case "Station":
			text = it.Name
		case "Display":
			text = it.Display
		}
		if catalog.Blank(text) {
			return fmt.Errorf("menu %q: %s item %d has no name", menu, it.Type, i+1)
		}
	}
	return nil
}

func (it item) label() string {
	switch it.Type {
	case "Dir":
		return fmt.Sprintf("[%s] (%d)", it.Title, it.Count)
	case "Station":
		return fmt.Sprintf("%s  <%s %s>", it.Name, it.Mime, it.StreamURL)
	case "Search":
		return "Search..."
	case "Display":
		return "(" + it.Display + ")"
	}
	return it.Type
}

type receiver struct {
	client *http.Client
	host   string
	mac    string
	page   int
}

func newReceiver(server, host, mac string) *receiver {
	d := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		// Like a receiver whose DNS points at the server: every *.vtuner.com name goes to -server.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			h, _, _ := net.SplitHostPort(addr)
			if h == "vtuner.com" || strings.HasSuffix(h, ".vtuner.com") {
				addr = server
			}
			return d.DialContext(ctx, network, addr)
		},
		Proxy: nil,
	}
	return &receiver{client: &http.Client{Transport: tr, Timeout: 30 * time.Second}, host: host, mac: mac, page: 50}
}

func (r *receiver) get(u string) ([]byte, error) {
	resp, err := r.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: status %d", u, resp.StatusCode)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/xml") {
		return nil, fmt.Errorf("%s: content type %q", u, resp.Header.Get("Content-Type"))
	}
	return io.ReadAll(resp.Body)
}

// list fetches u page by page, like a receiver, using startItems/endItems.
func (r *receiver) list(u string) ([]item, error) {
	var all []item
	for start := 1; ; start += r.page {
		sep := "&"
		if !strings.Contains(u, "?") {
			sep = "?"
		}
		b, err := r.get(fmt.Sprintf("%s%sstartItems=%d&endItems=%d", u, sep, start, start+r.page-1))
		if err != nil {
			return nil, err
		}
		var l list
		if err := xml.Unmarshal(b, &l); err != nil {
			return nil, fmt.Errorf("bad XML from %s: %w", u, err)
		}
		all = append(all, l.Items...)
		if len(l.Items) == 0 || len(all) >= l.Count {
			return all, nil
		}
	}
}

func (r *receiver) login() ([]item, error) {
	base := "http://" + r.host + "/setupapp/denon/asp/BrowseXML/loginXML.asp"
	b, err := r.get(base + "?token=0")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(b), "<EncryptedToken>") {
		return nil, fmt.Errorf("unexpected token answer: %s", b)
	}
	return r.list(base + "?gofile=&mac=" + url.QueryEscape(r.mac) + "&dlang=eng&fver=1")
}

// play reads the first bytes of a stream.
func (r *receiver) play(u string, n int64) (string, int64, error) {
	resp, err := r.client.Get(u)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	got, err := io.Copy(io.Discard, io.LimitReader(resp.Body, n))
	return resp.Header.Get("Content-Type"), got, err
}

func main() {
	server := flag.String("server", "127.0.0.1:80", "address of the vTuner server (or the proxy in front of it)")
	host := flag.String("host", "denon.vtuner.com", "vTuner host name the receiver uses")
	mac := flag.String("mac", "00:11:22:33:44:55", "receiver ID sent at login")
	auto := flag.Bool("auto", false, "walk the menus automatically and exit non-zero on errors")
	noDir := flag.Bool("no-directory", false, "with -auto: skip the public directory")
	flag.Parse()

	r := newReceiver(*server, *host, *mac)
	var err error
	if *auto {
		err = smoke(r, *noDir)
	} else {
		err = interactive(r)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func show(items []item) {
	for i, it := range items {
		fmt.Printf("%3d  %s\n", i+1, it.label())
	}
}

func interactive(r *receiver) error {
	items, err := r.login()
	if err != nil {
		return err
	}
	var stack [][]item
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println()
		show(items)
		fmt.Print("\nNumber to open, b = back, q = quit: ")
		if !in.Scan() {
			return nil
		}
		cmd := strings.TrimSpace(in.Text())
		switch cmd {
		case "q":
			return nil
		case "b":
			if len(stack) > 0 {
				items, stack = stack[len(stack)-1], stack[:len(stack)-1]
			}
			continue
		}
		n, err := strconv.Atoi(cmd)
		if err != nil || n < 1 || n > len(items) {
			fmt.Println("?")
			continue
		}
		it := items[n-1]
		var next []item
		switch it.Type {
		case "Dir":
			next, err = r.list(it.URLDir)
		case "Search":
			fmt.Print("Search for: ")
			if !in.Scan() {
				return nil
			}
			next, err = r.list(it.SearchURL + url.QueryEscape(in.Text()))
		case "Station":
			ct, got, perr := r.play(it.StreamURL, 64<<10)
			if perr != nil {
				fmt.Println("play failed:", perr)
			} else {
				fmt.Printf("playing: got %d bytes of %s\n", got, ct)
			}
			continue
		default:
			continue
		}
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		stack = append(stack, items)
		items = next
	}
}

func find(items []item, typ, title string) (item, bool) {
	for _, it := range items {
		if it.Type == typ && (title == "" || it.Title == title) {
			return it, true
		}
	}
	return item{}, false
}

func smoke(r *receiver, noDir bool) error {
	step := func(name string) { fmt.Println("==", name) }
	step("login")
	root, err := r.login()
	if err != nil {
		return err
	}
	show(root)
	if len(root) == 0 {
		return errors.New("root menu is empty")
	}
	if err := checkNames("root", root); err != nil {
		return err
	}
	var played bool
	tryPlay := func(items []item) {
		for _, it := range items {
			if it.Type != "Station" || played {
				continue
			}
			ct, n, err := r.play(it.StreamURL, 16<<10)
			if err != nil {
				fmt.Printf("   could not play %q: %v\n", it.Name, err)
				continue
			}
			fmt.Printf("   played %q: %d bytes of %s\n", it.Name, n, ct)
			played = true
		}
	}
	var walk func(items []item) error
	walk = func(items []item) error {
		for _, it := range items {
			if it.Type != "Dir" || strings.HasPrefix(it.Title, "Browse by") {
				continue
			}
			step("folder " + it.Title)
			sub, err := r.list(it.URLDir)
			if err != nil {
				return err
			}
			show(sub)
			if err := checkNames(it.Title, sub); err != nil {
				return err
			}
			if err := walk(sub); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return err
	}
	if s, ok := find(root, "Search", ""); ok {
		step("search \"example\"")
		items, err := r.list(s.SearchURL + "example")
		if err != nil {
			return err
		}
		fmt.Printf("   %d results\n", len(items))
	}
	if !noDir {
		c, ok := find(root, "Dir", "Browse by country")
		if !ok {
			return errors.New("no Browse by country entry (use -no-directory if the directory is off)")
		}
		step("browse by country")
		countries, err := r.list(c.URLDir)
		if err != nil {
			return err
		}
		fmt.Printf("   %d countries\n", len(countries))
		if len(countries) == 0 || countries[0].Type != "Dir" {
			return errors.New("public directory returned no countries")
		}
		if err := checkNames("Browse by country", countries); err != nil {
			return err
		}
		if g, ok := find(root, "Dir", "Browse by genre"); ok {
			step("browse by genre")
			genres, err := r.list(g.URLDir)
			if err != nil {
				return err
			}
			fmt.Printf("   %d genres\n", len(genres))
			if err := checkNames("Browse by genre", genres); err != nil {
				return err
			}
		}
		pick := countries[0]
		for _, x := range countries {
			if x.Title == "Germany" {
				pick = x
			}
		}
		step("country " + pick.Title)
		st, err := r.list(pick.URLDir)
		if err != nil {
			return err
		}
		fmt.Printf("   %d stations\n", len(st))
		if err := checkNames(pick.Title, st); err != nil {
			return err
		}
		if len(st) > 5 {
			show(st[:5])
		}
		tryPlay(st)
		if !played {
			return errors.New("could not play any directory station")
		}
	}
	fmt.Println("== OK")
	return nil
}
