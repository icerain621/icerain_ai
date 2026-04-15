package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func main() {
	var (
		addr   = flag.String("addr", "127.0.0.1:18080", "listen address")
		origin = flag.String("origin", "", "public origin shown on pages (optional)")
	)
	flag.Parse()

	logger := log.New(log.Writer(), "", log.LstdFlags|log.Lmicroseconds)

	mux := http.NewServeMux()
	s := &site{origin: *origin}

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/app", s.handleApp)
	mux.HandleFunc("/api/profile", s.handleAPIProfile)

	server := &http.Server{
		Addr:              *addr,
		Handler:           logging(mux, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Fatalf("listen: %v", err)
	}
	logger.Printf("mocksite listening on http://%s", ln.Addr().String())
	logger.Printf("login page: http://%s/login", ln.Addr().String())
	logger.Printf("app page:   http://%s/app", ln.Addr().String())
	logger.Fatal(server.Serve(ln))
}

type site struct {
	origin string
}

func (s *site) baseURL(r *http.Request) string {
	if s.origin != "" {
		return strings.TrimRight(s.origin, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (s *site) isLoggedIn(r *http.Request) bool {
	c, err := r.Cookie("mock_session")
	return err == nil && c.Value != ""
}

func (s *site) requireLogin(w http.ResponseWriter, r *http.Request) bool {
	if s.isLoggedIn(r) {
		return true
	}
	http.Redirect(w, r, "/login?next=/app", http.StatusFound)
	return false
}

func (s *site) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/app", http.StatusFound)
}

func (s *site) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		next := r.URL.Query().Get("next")
		if next == "" {
			next = "/app"
		}
		data := map[string]any{
			"BaseURL": s.baseURL(r),
			"Next":    next,
		}
		_ = loginTpl.Execute(w, data)
	case http.MethodPost:
		// No validation mechanism by design. Any username/password logs you in.
		_ = r.ParseForm()
		next := r.Form.Get("next")
		if next == "" {
			next = "/app"
		}
		sid := newSessionID()
		http.SetCookie(w, &http.Cookie{
			Name:     "mock_session",
			Value:    sid,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, next, http.StatusFound)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *site) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mock_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *site) handleApp(w http.ResponseWriter, r *http.Request) {
	if !s.requireLogin(w, r) {
		return
	}
	data := map[string]any{
		"BaseURL": s.baseURL(r),
	}
	_ = appTpl.Execute(w, data)
}

func (s *site) handleAPIProfile(w http.ResponseWriter, r *http.Request) {
	if !s.isLoggedIn(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"user":{"login":"demo-user","plan":"free"}}`))
}

func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func logging(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

var loginTpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Sign in · MockSite</title>
  <style>
    :root { --bg:#0d1117; --fg:#c9d1d9; --muted:#8b949e; --card:#161b22; --border:#30363d; --accent:#2f81f7; }
    body { margin:0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; background:var(--bg); color:var(--fg); }
    .wrap { max-width: 420px; margin: 64px auto; padding: 0 16px; }
    .logo { font-weight: 700; letter-spacing: .5px; margin-bottom: 16px; }
    .card { background:var(--card); border:1px solid var(--border); border-radius: 10px; padding: 20px; }
    .title { font-size: 20px; margin: 0 0 12px; }
    label { display:block; font-size: 12px; color:var(--muted); margin-top: 12px; }
    input { width:100%; box-sizing:border-box; margin-top: 6px; padding: 10px 12px; border-radius: 8px; border:1px solid var(--border); background:#0b0f14; color:var(--fg); }
    button { margin-top: 16px; width:100%; padding: 10px 12px; border-radius: 8px; border: 1px solid var(--accent); background: var(--accent); color: white; font-weight: 600; cursor: pointer; }
    .hint { margin-top: 12px; font-size: 12px; color: var(--muted); line-height: 1.5; }
    a { color: var(--accent); text-decoration: none; }
    .meta { margin-top: 10px; font-size: 12px; color: var(--muted); }
    .topbar { position: fixed; top:0; left:0; right:0; border-bottom: 1px solid var(--border); background: rgba(13,17,23,.8); backdrop-filter: blur(6px); }
    .topbar .inner { max-width: 980px; margin: 0 auto; padding: 10px 16px; color: var(--muted); font-size: 12px; }
  </style>
</head>
<body>
  <div class="topbar"><div class="inner">MockSite · local demo for chromedp login</div></div>
  <div class="wrap">
    <div class="logo">MockSite</div>
    <div class="card">
      <h1 class="title">Sign in</h1>
      <form method="post" action="/login">
        <input type="hidden" name="next" value="{{.Next}}"/>
        <label for="login_field">Username or email address</label>
        <input id="login_field" name="login" autocomplete="username" />
        <label for="password">Password</label>
        <input id="password" name="password" type="password" autocomplete="current-password" />
        <button type="submit" id="sign_in">Sign in</button>
      </form>
      <div class="hint">
        This page intentionally has <strong>no validation</strong>. Any credentials will set a session cookie and redirect.
      </div>
      <div class="meta">Base URL: <code>{{.BaseURL}}</code></div>
    </div>
  </div>
</body>
</html>`))

var appTpl = template.Must(template.New("app").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>App · MockSite</title>
  <style>
    :root { --bg:#0d1117; --fg:#c9d1d9; --muted:#8b949e; --card:#161b22; --border:#30363d; --accent:#2f81f7; }
    body { margin:0; font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; background:var(--bg); color:var(--fg); }
    .wrap { max-width: 980px; margin: 48px auto; padding: 0 16px; }
    .card { background:var(--card); border:1px solid var(--border); border-radius: 10px; padding: 16px; margin-top: 16px; }
    .row { display:flex; gap: 12px; flex-wrap: wrap; }
    .pill { display:inline-block; padding: 6px 10px; border-radius: 999px; border:1px solid var(--border); color: var(--muted); font-size: 12px; }
    a { color: var(--accent); text-decoration: none; }
  </style>
</head>
<body>
  <div class="wrap">
    <h1 id="app_home">Welcome</h1>
    <div class="row">
      <span class="pill">logged_in=true</span>
      <span class="pill">api: <code>{{.BaseURL}}/api/profile</code></span>
      <span class="pill"><a id="logout_link" href="/logout">logout</a></span>
    </div>
    <div class="card">
      <h2>Data</h2>
      <p>This page simulates a post-login app.</p>
      <p>Try calling <code>/api/profile</code> (XHR/JSON).</p>
    </div>
  </div>
</body>
</html>`))

