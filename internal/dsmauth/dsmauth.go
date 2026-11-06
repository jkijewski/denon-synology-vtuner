// Package dsmauth runs the admin UI as a DSM CGI: it checks the caller's DSM login
// and forwards allowed requests to the server with the shared admin token.
package dsmauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/http/cgi"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Paths of DSM's own CGIs, as documented for DSM third-party apps.
var (
	LoginCGI        = "/usr/syno/synoman/webman/login.cgi"
	AuthenticateCGI = "/usr/syno/synoman/webman/modules/authenticate.cgi"
	AdminGroup      = "administrators"
)

const cgiTimeout = 10 * time.Second

var tokenRE = regexp.MustCompile(`"SynoToken"\s*:\s*"([^"]*)"`)

// trace records what the login check saw, to explain a refusal on a real NAS. It must
// never hold a cookie, a SynoToken or the output of a DSM CGI, because a refused visitor
// can read it.
type trace []string

func (t *trace) addf(format string, args ...any) { *t = append(*t, fmt.Sprintf(format, args...)) }

// describeFile reports a file's mode and owner, which decide whether this process can run it.
func describeFile(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "cannot be read: " + reason(err)
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("mode %04o, owner %d:%d", st.Mode().Perm(), sys.Uid, sys.Gid)
	}
	return fmt.Sprintf("mode %04o", st.Mode().Perm())
}

// describeRun reports how a command ended, without its output.
func describeRun(err error) string {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return "exit 0"
	case errors.As(err, &exit):
		return fmt.Sprintf("exit %d", exit.ExitCode())
	default:
		return reason(err)
	}
}

// reason drops the path from an error, keeping the cause such as "permission denied".
func reason(err error) string {
	if inner := errors.Unwrap(err); inner != nil {
		return inner.Error()
	}
	return err.Error()
}

// runCGI runs a DSM CGI with the environment of this request, as a GET with the given query:
// the method and body of the browser's request are for vTuner Radio, not for DSM's CGI.
func runCGI(path, query string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cgiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Env = append(os.Environ(), "REQUEST_METHOD=GET", "CONTENT_LENGTH=", "CONTENT_TYPE=", "QUERY_STRING="+query)
	return cmd.Output()
}

// requestToken returns a SynoToken that the browser sent with this request.
func requestToken() string {
	if t := os.Getenv("HTTP_X_SYNO_TOKEN"); t != "" {
		return t
	}
	q, _ := url.ParseQuery(os.Getenv("QUERY_STRING"))
	return q.Get("SynoToken")
}

// CurrentUser returns the DSM user of the browser session that called this CGI.
// It uses the CGI environment (cookies) that DSM passes to the process.
func CurrentUser() (string, error) {
	var t trace
	return currentUser(&t)
}

func currentUser(t *trace) (string, error) {
	t.addf("running as uid %d, gid %d, groups %v", os.Getuid(), os.Getgid(), groups())
	cookie := "no"
	if os.Getenv("HTTP_COOKIE") != "" {
		cookie = "yes"
	}
	t.addf("browser sent cookies: %s", cookie)

	token := requestToken()
	if token != "" {
		t.addf("SynoToken: from the request")
	} else {
		t.addf("login.cgi: %s", describeFile(LoginCGI))
		out, err := runCGI(LoginCGI, "")
		m := tokenRE.FindSubmatch(out)
		found := "no"
		if m != nil && len(m[1]) > 0 {
			token, found = string(m[1]), "yes"
		}
		// A missing token isn't final: DSM needs none when its CSRF protection is off.
		t.addf("login.cgi run: %s, SynoToken found: %s", describeRun(err), found)
	}

	query := ""
	if token != "" {
		query = "SynoToken=" + url.QueryEscape(token)
	}
	t.addf("authenticate.cgi: %s", describeFile(AuthenticateCGI))
	out, err := runCGI(AuthenticateCGI, query)
	t.addf("authenticate.cgi run: %s, output: %d bytes", describeRun(err), len(out))
	if err != nil {
		return "", fmt.Errorf("authenticate.cgi: %w", err)
	}
	user := strings.TrimSpace(string(out))
	if user == "" || strings.HasPrefix(user, "-") || strings.ContainsAny(user, " \t\n/") {
		return "", errors.New("not logged in")
	}
	return user, nil
}

func groups() []int {
	g, err := os.Getgroups()
	if err != nil {
		return nil
	}
	return g
}

// idPath finds the id command, as the PATH of a CGI can be minimal.
func idPath() string {
	if p, err := exec.LookPath("id"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/bin/id", "/bin/id"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "id"
}

// IsAdmin reports whether user is in the DSM administrators group.
func IsAdmin(user string) bool {
	out, err := exec.Command(idPath(), "-Gn", user).Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Fields(string(out)) {
		if g == AdminGroup {
			return true
		}
	}
	return false
}

// Serve handles one CGI request: authenticate, then proxy PATH_INFO to backend/admin with the token.
func Serve(backend, token, tokenHeader string) error {
	target, err := url.Parse(backend)
	if err != nil {
		return err
	}
	return cgi.Serve(Handler(target, token, tokenHeader))
}

// Handler does the work of Serve. The DSM login is a cookie that the browser sends with
// requests from any site, and CurrentUser gets the SynoToken itself, so the token doesn't
// show that the request came from the admin page. Requests that change something from
// another site are therefore refused first.
func Handler(target *url.URL, token, tokenHeader string) http.Handler {
	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		deny(w, http.StatusForbidden, "This request came from another site and was refused.")
	}))
	return csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var t trace
		user, err := currentUser(&t)
		if err != nil {
			log.Printf("DSM login check failed: %v; %s", err, strings.Join(t, "; "))
			if os.Getenv("HTTP_COOKIE") == "" {
				deny(w, http.StatusUnauthorized, "Please sign in to DSM first, then open vTuner Radio from the DSM main menu.", t...)
				return
			}
			deny(w, http.StatusUnauthorized, "vTuner Radio couldn't confirm your DSM login. If you're signed in to DSM, this is a problem with the package, not your account. Please report the details below.", t...)
			return
		}
		if !IsAdmin(user) {
			deny(w, http.StatusForbidden, "Only DSM administrators can manage vTuner Radio.")
			return
		}
		pathInfo := os.Getenv("PATH_INFO")
		if pathInfo == "" {
			// Make relative URLs in the page resolve under index.cgi/.
			http.Redirect(w, r, os.Getenv("SCRIPT_NAME")+"/", http.StatusFound)
			return
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.Out.URL.Scheme, pr.Out.URL.Host = target.Scheme, target.Host
				pr.Out.URL.Path = "/admin" + pathInfo
				pr.Out.URL.RawPath = ""
				pr.Out.URL.RawQuery = r.URL.RawQuery
				pr.Out.Host = target.Host
				pr.Out.Header.Del("Cookie")
				pr.Out.Header.Set(tokenHeader, token)
			},
		}
		proxy.ServeHTTP(w, r)
	}))
}

func deny(w http.ResponseWriter, code int, msg string, details ...string) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<!doctype html><meta charset=utf-8><title>vTuner Radio</title><div style='font:15px system-ui;margin:2em'><p>%s</p>", html.EscapeString(msg))
	if len(details) > 0 {
		fmt.Fprintf(&b, "<details><summary>Details</summary><pre>%s</pre></details>", html.EscapeString(strings.Join(details, "\n")))
	}
	b.WriteString("</div>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	w.Write(b.Bytes())
}
