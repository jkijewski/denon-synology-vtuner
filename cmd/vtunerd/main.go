// Command vtunerd is a self-hosted replacement for the vTuner internet radio directory.
//
//	vtunerd serve   run the server (default)
//	vtunerd cgi     run as the DSM admin UI CGI (called by DSM's web server)
//	vtunerd schema-version  print the data version this build uses (for the package build)
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"denon-synology-vtuner/internal/admin"
	"denon-synology-vtuner/internal/devices"
	"denon-synology-vtuner/internal/directory"
	"denon-synology-vtuner/internal/dsmauth"
	"denon-synology-vtuner/internal/logfile"
	"denon-synology-vtuner/internal/relay"
	"denon-synology-vtuner/internal/reqlog"
	"denon-synology-vtuner/internal/schema"
	"denon-synology-vtuner/internal/vtuner"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "cgi":
		err = runCGI(args)
	case "version":
		fmt.Println(version)
	case "schema-version":
		fmt.Println(schema.Current)
	default:
		err = fmt.Errorf("unknown command %q (use serve, cgi, version or schema-version)", cmd)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", env("VTUNER_LISTEN", "127.0.0.1:8580"), "address to listen on")
	dataDir := fs.String("data", env("VTUNER_DATA", "./data"), "data folder (station lists, settings, logs)")
	host := fs.String("host", env("VTUNER_HOST", "denon.vtuner.com"), "vTuner host name used in generated URLs")
	maxTitle := fs.Int("max-title", 40, "shorten names longer than this many characters")
	relayMax := fs.Int("relay-max", 3, "maximum simultaneous relayed streams")
	logMax := fs.Int("request-log", 500, "number of receiver requests kept for the admin UI")
	dirURL := fs.String("directory", env("VTUNER_DIRECTORY", "auto"), "public directory API base URL, \"auto\" to discover radio-browser servers, or \"off\"")
	dirLimit := fs.Int("directory-limit", 200, "maximum stations per directory listing")
	logPath := fs.String("log", "", "write the service log to this file (rotated); default stderr")
	search := fs.Bool("search", false, "offer a Search entry in the receiver's top menu (the Denon RCD-N7 shows it as a row with no name)")
	dev := fs.Bool("dev", false, "serve the admin UI without a token (only allowed on a loopback address)")
	fs.Parse(args)

	if *logPath != "" {
		log.SetOutput(&logfile.Writer{Path: *logPath, MaxBytes: 1 << 20, Keep: 3})
	}
	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		return err
	}
	// Before anything else reads the data. On failure the app stays stopped, so that it
	// can't overwrite data it doesn't understand.
	if err := schema.UpgradeFolder(*dataDir); err != nil {
		return err
	}
	token := ""
	if *dev {
		h, _, err := net.SplitHostPort(*listen)
		if ip := net.ParseIP(h); err != nil || ip == nil || !ip.IsLoopback() {
			return errors.New("-dev needs a loopback listen address such as 127.0.0.1:8580")
		}
		log.Printf("dev mode: admin UI at http://%s/admin/ without login", *listen)
	} else {
		var err error
		if token, err = loadOrCreateToken(filepath.Join(*dataDir, "admin.token")); err != nil {
			return err
		}
	}

	reg, err := devices.Open(*dataDir)
	if err != nil {
		return err
	}
	var dir *directory.Client
	if *dirURL != "off" {
		base := *dirURL
		if base == "auto" {
			base = ""
		}
		dir = directory.New(base)
		dir.UserAgent = "denon-synology-vtuner/" + version
	}
	rlog := reqlog.New(*logMax)
	vt := &vtuner.Handler{Devices: reg, Directory: dir, Log: rlog, DefaultHost: *host, MaxTitle: *maxTitle,
		SearchEnabled: *search, Token: "0000000000000000", DirLimit: *dirLimit, TagCount: 60}
	rl := relay.New(vt.RelayResolver, *relayMax)
	adm := &admin.Handler{Devices: reg, Directory: dir, DirLimit: *dirLimit, Log: rlog, Relay: rl, Token: token, ServiceLog: *logPath, Version: version}

	mux := http.NewServeMux()
	mux.Handle("/admin/", adm)
	mux.Handle("/relay/", rl)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok\n") })
	mux.Handle("/", vt)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	// No WriteTimeout, because relayed streams run for hours; the relay sets its own deadlines.
	// Request contexts derive from ctx, so stopping the package also ends the streams.
	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	log.Printf("vtunerd %s listening on %s, data in %s", version, *listen, *dataDir)
	failed := make(chan error, 1)
	go func() { failed <- srv.ListenAndServe() }()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	log.Print("stopping")
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

// loadOrCreateToken reads the shared admin token, creating a random one on first start.
func loadOrCreateToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(b))) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

// runCGI reads the port and token from the data folder and serves one CGI request.
func runCGI(args []string) error {
	fs := flag.NewFlagSet("cgi", flag.ExitOnError)
	dataDir := fs.String("data", env("VTUNER_DATA", "/var/packages/vtunerradio/var"), "data folder")
	fs.Parse(args)
	tok, err := os.ReadFile(filepath.Join(*dataDir, "admin.token"))
	if err != nil {
		return err
	}
	listen := settings(filepath.Join(*dataDir, "settings.env"))["VTUNER_LISTEN"]
	if listen == "" {
		listen = "127.0.0.1:8580"
	}
	return dsmauth.Serve("http://"+listen, strings.TrimSpace(string(tok)), admin.TokenHeader)
}

// settings parses KEY=value lines.
func settings(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if k, v, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") {
			out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return out
}
