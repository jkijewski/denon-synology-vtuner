package dsmauth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func fake(t *testing.T, dir, name, body string) string {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCurrentUser(t *testing.T) {
	dir := t.TempDir()
	LoginCGI = fake(t, dir, "login.cgi", `printf 'Content-Type: application/json\n\n{"SynoToken" : "abc123", "result":"success"}'`)
	AuthenticateCGI = fake(t, dir, "authenticate.cgi", `[ "$QUERY_STRING" = "SynoToken=abc123" ] && echo exampleadmin`)
	u, err := CurrentUser()
	if err != nil || u != "exampleadmin" {
		t.Fatalf("%q %v", u, err)
	}
	LoginCGI = fake(t, dir, "login2.cgi", `printf 'Content-Type: application/json\n\n{"result":"fail"}'`)
	if _, err := CurrentUser(); err == nil {
		t.Fatal("want error without session")
	}
}

func TestIsAdmin(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	g, err := user.LookupGroupId(me.Gid)
	if err != nil {
		t.Skip(err)
	}
	AdminGroup = g.Name
	if !IsAdmin(me.Username) {
		t.Fatal("own primary group not found")
	}
	AdminGroup = "no-such-group-example"
	if IsAdmin(me.Username) {
		t.Fatal("should not be admin")
	}
}

// handlerSetup installs fake DSM CGIs logging in as login, and a backend that records the last request.
func handlerSetup(t *testing.T, login string) (http.Handler, *http.Request) {
	t.Helper()
	me, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	g, err := user.LookupGroupId(me.Gid)
	if err != nil {
		t.Skip(err)
	}
	if login == "self" {
		login = me.Username
	}
	oldLogin, oldAuth, oldGroup := LoginCGI, AuthenticateCGI, AdminGroup
	t.Cleanup(func() { LoginCGI, AuthenticateCGI, AdminGroup = oldLogin, oldAuth, oldGroup })
	dir := t.TempDir()
	LoginCGI = fake(t, dir, "login.cgi", `printf 'Content-Type: application/json\n\n{"SynoToken":"abc"}'`)
	AuthenticateCGI = fake(t, dir, "authenticate.cgi", "echo "+login)
	AdminGroup = g.Name
	t.Setenv("PATH_INFO", "/api/catalog")

	seen := &http.Request{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r
		io.WriteString(w, "from backend")
	}))
	t.Cleanup(backend.Close)
	target, _ := url.Parse(backend.URL)
	return Handler(target, "secret-token", "X-Token"), seen
}

func TestHandlerForwardsAdminWithTokenAndNoCookie(t *testing.T) {
	h, seen := handlerSetup(t, "self")
	r := httptest.NewRequest("PUT", "http://nas.example.lan:5000/webman/index.cgi/api/catalog?device=default", strings.NewReader("{}"))
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Cookie", "id=dsm-session")
	r.Header.Set("X-Token", "spoofed")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "from backend" {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	if seen.URL.Path != "/admin/api/catalog" || seen.URL.RawQuery != "device=default" {
		t.Fatalf("%s", seen.URL)
	}
	if seen.Header.Get("X-Token") != "secret-token" || seen.Header.Get("Cookie") != "" {
		t.Fatalf("%v", seen.Header)
	}
}

func TestHandlerRefusals(t *testing.T) {
	for _, c := range []struct {
		name, login, method string
		headers             map[string]string
		want                int
	}{
		{"other site by fetch metadata", "self", "POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"other site by origin", "self", "POST", map[string]string{"Origin": "http://evil.example.com"}, 403},
		{"not logged in", "", "GET", nil, 401},
		{"not an administrator", "no-such-user-example", "GET", nil, 403},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, seen := handlerSetup(t, c.login)
			r := httptest.NewRequest(c.method, "http://nas.example.lan:5000/x", nil)
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != c.want {
				t.Fatalf("%d %q", w.Code, w.Body.String())
			}
			if seen.URL != nil {
				t.Fatal("request reached the backend")
			}
		})
	}
}

func TestHandlerAllowsSameOriginWithoutFetchMetadata(t *testing.T) {
	// Browsers omit Sec-Fetch-Site on plain HTTP, which is how DSM is often reached.
	h, seen := handlerSetup(t, "self")
	r := httptest.NewRequest("POST", "http://nas.example.lan:5000/x", nil)
	r.Header.Set("Origin", "http://nas.example.lan:5000")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || seen.URL == nil {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHandlerRedirectsWithoutPathInfo(t *testing.T) {
	h, _ := handlerSetup(t, "self")
	t.Setenv("PATH_INFO", "")
	t.Setenv("SCRIPT_NAME", "/webman/3rdparty/vtunerradio/index.cgi")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != http.StatusFound || !strings.HasSuffix(w.Header().Get("Location"), "index.cgi/") {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
}

// fakeSession installs fake DSM CGIs for one test and restores the real paths afterwards.
func fakeSession(t *testing.T, login, auth string) {
	t.Helper()
	oldLogin, oldAuth := LoginCGI, AuthenticateCGI
	t.Cleanup(func() { LoginCGI, AuthenticateCGI = oldLogin, oldAuth })
	dir := t.TempDir()
	LoginCGI = fake(t, dir, "login.cgi", login)
	AuthenticateCGI = fake(t, dir, "authenticate.cgi", auth)
	t.Setenv("QUERY_STRING", "")
	t.Setenv("HTTP_X_SYNO_TOKEN", "")
}

const printToken = `printf '{"SynoToken" : "abc123"}'`

func TestTokenFromRequestWithoutLoginCGI(t *testing.T) {
	// On DSM 7.x login.cgi can be closed to package users.
	fakeSession(t, "exit 1", `[ "$QUERY_STRING" = "SynoToken=fromurl" ] && echo exampleadmin`)
	if err := os.Chmod(LoginCGI, 0); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QUERY_STRING", "SynoToken=fromurl")
	if u, err := CurrentUser(); err != nil || u != "exampleadmin" {
		t.Fatalf("%q %v", u, err)
	}
	t.Setenv("QUERY_STRING", "")
	t.Setenv("HTTP_X_SYNO_TOKEN", "fromurl")
	if u, err := CurrentUser(); err != nil || u != "exampleadmin" {
		t.Fatalf("header: %q %v", u, err)
	}
}

func TestLoginWorksWithoutAnyToken(t *testing.T) {
	// With DSM's CSRF protection off there is no token, and authenticate.cgi still answers.
	for name, login := range map[string]string{
		"login.cgi fails": "exit 5",
		"empty token":     `printf '{"SynoToken" : ""}'`,
		"no token":        `printf '{"success":true}'`,
	} {
		t.Run(name, func(t *testing.T) {
			fakeSession(t, login, `[ -z "$QUERY_STRING" ] && echo exampleadmin`)
			if u, err := CurrentUser(); err != nil || u != "exampleadmin" {
				t.Fatalf("%q %v", u, err)
			}
		})
	}
}

func TestFailuresAreRefused(t *testing.T) {
	for name, auth := range map[string]string{
		"exit 5, as for a package user": "exit 5",
		"no output":                     "exit 0",
		"non-zero with output":          "echo exampleadmin; exit 1",
		"error text":                    "echo Permission denied",
		"option-like name":              "echo -G",
	} {
		t.Run(name, func(t *testing.T) {
			fakeSession(t, printToken, auth)
			if u, err := CurrentUser(); err == nil {
				t.Fatalf("accepted %q", u)
			}
		})
	}
}

func TestDSMCGIsSeeAGetWithoutTheBrowsersRequest(t *testing.T) {
	const check = `[ "$REQUEST_METHOD" = GET ] && [ -z "$CONTENT_LENGTH" ] || exit 9; `
	fakeSession(t, check+printToken, check+`[ "$QUERY_STRING" = "SynoToken=abc123" ] && echo exampleadmin`)
	t.Setenv("REQUEST_METHOD", "PUT")
	t.Setenv("CONTENT_LENGTH", "12")
	if u, err := CurrentUser(); err != nil || u != "exampleadmin" {
		t.Fatalf("%q %v", u, err)
	}
}

func TestRefusalExplainsWithoutSecrets(t *testing.T) {
	h, _ := handlerSetup(t, "")
	dir := t.TempDir()
	LoginCGI = fake(t, dir, "login2.cgi", `printf '{"SynoToken":"token-value-xyz"}'; echo stray-login-output`)
	AuthenticateCGI = fake(t, dir, "auth2.cgi", "echo stray-auth-output; exit 5")
	t.Setenv("HTTP_COOKIE", "id=cookie-value-xyz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	body := w.Body.String()
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, secret := range []string{"token-value-xyz", "cookie-value-xyz", "stray-login-output", "stray-auth-output"} {
		if strings.Contains(body, secret) {
			t.Errorf("page shows %q: %s", secret, body)
		}
	}
	for _, want := range []string{"couldn&#39;t confirm", "authenticate.cgi run: exit 5", "SynoToken found: yes", "browser sent cookies: yes", "running as uid"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q: %s", want, body)
		}
	}
}

func TestRefusalWithoutCookieAsksToSignIn(t *testing.T) {
	h, _ := handlerSetup(t, "")
	t.Setenv("HTTP_COOKIE", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != 401 || !strings.Contains(w.Body.String(), "Please sign in") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestMissingCGIIsDescribed(t *testing.T) {
	fakeSession(t, printToken, "echo x")
	LoginCGI = "/nonexistent/login.cgi"
	AuthenticateCGI = "/nonexistent/authenticate.cgi"
	var tr trace
	if _, err := currentUser(&tr); err == nil {
		t.Fatal("accepted")
	}
	if got := strings.Join(tr, "\n"); !strings.Contains(got, "cannot be read: no such file") {
		t.Fatal(got)
	}
}
