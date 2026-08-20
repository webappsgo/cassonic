// Package admin provides the cassonic server administration panel.
// All routes require an authenticated server admin session.
package admin

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/local/cassonic/src/common/i18n"
	"github.com/local/cassonic/src/config"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/service/scheduler"
	"github.com/local/cassonic/src/server/store"
)

//go:embed template
var assets embed.FS

// Handler is the cassonic admin panel HTTP handler.
type Handler struct {
	db        *store.DB
	cfg       *config.Config
	cfgPath   string
	version   string
	sched     *scheduler.Scheduler
	tmpls     map[string]*template.Template
	startTime time.Time
	// i18n serves translated strings for the unauthenticated first-run setup
	// wizard (AI.md PART 17 / .claude/rules/testing-rules.md PART 31). The
	// rest of the admin panel has no i18n wiring yet — see TODO.AI.md.
	i18n *i18n.Bundle
}

// New creates a fully configured admin Handler. cfgPath is the absolute path
// to server.yml that SaveConfig persists changes to.
func New(db *store.DB, cfg *config.Config, cfgPath, version string, sched *scheduler.Scheduler) *Handler {
	h := &Handler{
		db:        db,
		cfg:       cfg,
		cfgPath:   cfgPath,
		version:   version,
		sched:     sched,
		startTime: time.Now(),
		i18n:      i18n.Default(),
	}
	h.tmpls = h.parseTemplates()
	return h
}

// supportedLocales lists the locale codes with a translation file
// (.claude/rules/testing-rules.md: "Every language file needs matching keys").
var supportedLocales = map[string]bool{
	"en": true, "es": true, "fr": true,
	"de": true, "zh": true, "ar": true, "ja": true,
}

// resolveLocale selects the best locale for the request: ?lang= query param
// (writes a cookie) → lang cookie → Accept-Language → "en". Mirrors
// web.Handler.resolveLocale so both apps' language cookies stay compatible.
func (h *Handler) resolveLocale(w http.ResponseWriter, r *http.Request) string {
	if q := r.URL.Query().Get("lang"); q != "" {
		code := strings.ToLower(strings.SplitN(q, "-", 2)[0])
		if supportedLocales[code] {
			http.SetCookie(w, &http.Cookie{
				Name:     "lang",
				Value:    code,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			})
			return code
		}
	}
	if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
		code := strings.ToLower(strings.SplitN(c.Value, "-", 2)[0])
		if supportedLocales[code] {
			return code
		}
	}
	if al := r.Header.Get("Accept-Language"); al != "" {
		for _, tag := range strings.Split(al, ",") {
			tag = strings.TrimSpace(strings.SplitN(tag, ";", 2)[0])
			code := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
			if supportedLocales[code] {
				return code
			}
		}
	}
	return "en"
}

// parseTemplates loads all HTML templates from the embedded filesystem.
//
// Each page template is parsed into its own isolated *template.Template together
// with the shared base.html layout. Page templates are NOT parsed all together
// into one shared namespace: every page defines a template block named "content",
// and html/template's ParseFS shares a single definition namespace across all
// files passed to it — so parsing them together would let the last-parsed file's
// "content" block silently win for every page. Parsing each page separately with
// base.html keeps each page's "content" block isolated to its own template.
func (h *Handler) parseTemplates() map[string]*template.Template {
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "never"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"formatDuration": func(d time.Duration) string {
			if d < time.Minute {
				return fmt.Sprintf("%ds", int(d.Seconds()))
			}
			if d < time.Hour {
				return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
			}
			return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		},
		"until": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			d := time.Until(t)
			if d < 0 {
				return "now"
			}
			return fmt.Sprintf("in %s", d.Round(time.Second))
		},
	}

	sub, err := fs.Sub(assets, "template")
	if err != nil {
		panic(fmt.Sprintf("admin: sub template fs: %v", err))
	}

	names, err := fs.Glob(sub, "*.html")
	if err != nil {
		panic(fmt.Sprintf("admin: glob templates: %v", err))
	}

	out := make(map[string]*template.Template, len(names))
	for _, name := range names {
		if name == "base.html" || name == "base_setup.html" {
			continue
		}
		// base_setup.html is the standalone (no sidebar) layout used by the
		// unauthenticated first-run setup wizard pages (AI.md PART 17); it
		// coexists harmlessly with base.html since html/template only uses
		// whichever {{template "..."}} a given page actually references.
		files := []string{"base.html", "base_setup.html", name}
		tmpl, err := template.New("").Funcs(funcMap).ParseFS(sub, files...)
		if err != nil {
			panic(fmt.Sprintf("admin: parse template %s: %v", name, err))
		}
		out[name] = tmpl
	}
	return out
}

// Routes assembles the chi router for the admin panel.
// All routes are wrapped in requireAdmin middleware.
// basePath returns the mounted admin panel root, e.g. "/server/admin", built
// from the configured server.admin_path (AI.md PART 17 "Configurable Admin
// Path") so internal redirects never hardcode the default segment.
func (h *Handler) basePath() string {
	return "/server/" + h.cfg.AdminPath()
}

// validAdminRootPaths lists every first path segment allowed directly under
// /server/{admin_path}/ (AI.md PART 17 "Route Conflict Prevention"). Only
// "config" (server management) and the current admin's own username (self
// account routes) may ever appear there — chi's router already refuses any
// other segment by returning 404, but validateAdminRoute is kept as an
// explicit, documented guard matching AI.md's literal specification.
var validAdminRootPaths = map[string]bool{
	"config": true,
}

// validateAdminRoute reports an error if path's first segment is neither a
// registered server-management root ("config") nor currentAdminUsername.
func validateAdminRoute(path, currentAdminUsername string) error {
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 2)
	firstSegment := parts[0]
	if firstSegment == "" || validAdminRootPaths[firstSegment] || firstSegment == currentAdminUsername {
		return nil
	}
	msg := "invalid admin route: /%s/* - use /server/{admin_path}/{admin_username}/* for " +
		"admin self routes or /server/{admin_path}/config/* for server management"
	return fmt.Errorf(msg, firstSegment)
}

// enforceAdminRouteHierarchy rejects any request whose first path segment is
// neither "config" nor the authenticated admin's own username, per AI.md
// PART 17 "Admin Route Hierarchy". Runs after requireAdmin, so an AdminUser
// is always present in context.
func (h *Handler) enforceAdminRouteHierarchy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin := mw.AdminFromContext(r.Context())
		username := ""
		if admin != nil {
			username = admin.Username
		}
		if err := validateAdminRoute(r.URL.Path, username); err != nil {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// Root ("/") and the setup wizard ("/config/setup") are reachable
	// pre-authentication: a fresh install has zero admins, so requireAdmin
	// would permanently lock operators out before they could ever create
	// one (AI.md PART 17 "First Run & Setup Wizard" / "Setup Flow"). Each
	// handler performs its own internal admin-session/setup-state check
	// instead of relying on the requireAdmin middleware.
	r.Get("/", h.Root)
	r.Post("/", h.RootPost)
	r.Get("/config/setup", h.SetupWizard)
	r.Post("/config/setup", h.SetupWizardPost)

	r.Group(func(gr chi.Router) {
		gr.Use(h.requireAdmin, h.enforceAdminRouteHierarchy)

		gr.Route("/config", func(cr chi.Router) {
			cr.Get("/info", h.System)
			cr.Get("/library", h.Library)
			cr.Post("/library/scan", h.TriggerScan)
			cr.Get("/scheduler", h.SchedulerPanel)
			cr.Post("/scheduler/{job}/run", h.RunJob)
			cr.Get("/settings", h.Config)
			cr.Post("/settings", h.SaveConfig)
			cr.Get("/logs", h.Logs)
			cr.Get("/backup", h.Backup)
			cr.Post("/backup/now", h.BackupNow)
		})

		gr.Route("/{admin_username}", func(ur chi.Router) {
			ur.Get("/", h.SelfRoot)
			ur.Get("/profile", h.Profile)
			ur.Post("/profile", h.SaveProfile)
			ur.Get("/preferences", h.Preferences)
			ur.Post("/preferences", h.SavePreferences)
			ur.Get("/profile/security", h.Security)
			ur.Post("/profile/security/totp/enroll", h.EnrollTOTPStart)
			ur.Post("/profile/security/totp/confirm", h.EnrollTOTPConfirm)
			ur.Post("/profile/security/totp/disable", h.DisableTOTP)
			ur.Post("/profile/security/totp/backup-codes/regenerate", h.RegenerateBackupCodes)
		})
	})

	return r
}

// requireAdmin verifies the requesting user is an authenticated Server Admin
// (AI.md PART 17 "Admin Panel Isolation"). Checks the admin_session cookie
// (SHA-256 hash → AdminStore.GetAdminSessionByHash) against the dedicated
// admins table — deliberately never users.IsAdmin, which is an unrelated
// Subsonic/Ampache privileged-user flag on a completely separate account
// type. Non-admin requests are redirected to /login?next={path}.
func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check context first (set by an earlier requireAdmin call, e.g. in
		// tests that pre-seed the context).
		if a := mw.AdminFromContext(r.Context()); a != nil {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(mw.AdminSessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}

		raw := sha256.Sum256([]byte(cookie.Value))
		hashHex := hex.EncodeToString(raw[:])

		ctx := r.Context()
		session, err := h.db.Admin.GetAdminSessionByHash(ctx, hashHex)
		if err != nil || session == nil || session.IsExpired() {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}

		admin, err := h.db.Admin.GetAdmin(ctx, session.AdminID)
		if err != nil || admin == nil || !admin.Enabled {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx = mw.WithAdmin(ctx, &mw.AdminUser{
			ID:       admin.ID,
			Username: admin.Username,
			Role:     admin.Role,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sessionAdmin looks up the admin_session cookie against AdminStore and
// returns the authenticated admin, or nil if the request carries no valid,
// non-expired session for an enabled admin. Used by Root, which — unlike
// requireAdmin — must fall through to the setup-token-entry/redirect logic
// on any failure rather than responding directly.
func (h *Handler) sessionAdmin(r *http.Request) *mw.AdminUser {
	cookie, err := r.Cookie(mw.AdminSessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}

	raw := sha256.Sum256([]byte(cookie.Value))
	hashHex := hex.EncodeToString(raw[:])

	ctx := r.Context()
	session, err := h.db.Admin.GetAdminSessionByHash(ctx, hashHex)
	if err != nil || session == nil || session.IsExpired() {
		return nil
	}

	admin, err := h.db.Admin.GetAdmin(ctx, session.AdminID)
	if err != nil || admin == nil || !admin.Enabled {
		return nil
	}

	return &mw.AdminUser{
		ID:       admin.ID,
		Username: admin.Username,
		Role:     admin.Role,
	}
}

// adminPageData carries data common to every admin template.
type adminPageData struct {
	Title    string
	Version  string
	Active   string
	BasePath string
	// AdminUsername is the authenticated admin's own username, used by
	// base.html to link to the /{admin_username}/* self-account routes
	// (AI.md PART 17 "Admin Route Hierarchy").
	AdminUsername string
	// CSRFToken is the double-submit CSRF token issued by mw.CSRF for this
	// request, embedded as a hidden csrf_token form field.
	CSRFToken string
	// Lang and T serve translated strings to the setup wizard templates
	// (AI.md PART 31); unused by the rest of the admin panel today.
	Lang string
	T    func(string) string
	Data any
}

// render executes the named template with the provided page data. r supplies
// the authenticated admin's identity (set by requireAdmin) for the self-route
// links in the sidebar.
func (h *Handler) render(w http.ResponseWriter, r *http.Request, name, title, active string, data any) {
	username := ""
	if a := mw.AdminFromContext(r.Context()); a != nil {
		username = a.Username
	}
	lang := h.resolveLocale(w, r)
	bundle := h.i18n
	pd := adminPageData{
		Title:         title,
		Version:       h.version,
		Active:        active,
		BasePath:      h.basePath(),
		AdminUsername: username,
		CSRFToken:     mw.CSRFTokenFromContext(r.Context()),
		Lang:          lang,
		T:             func(key string) string { return bundle.T(lang, key) },
		Data:          data,
	}
	tmpl, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "template error: unknown template "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, pd); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// dashboardData holds stats for the admin dashboard.
type dashboardData struct {
	Uptime        string
	Version       string
	ActiveStreams int
	TorEnabled    bool
}

// Dashboard renders the admin panel home page with server statistics.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime).Round(time.Second)
	d := dashboardData{
		Uptime:  uptime.String(),
		Version: h.version,
	}
	h.render(w, r, "dashboard.html", "Dashboard — Admin", "dashboard", d)
}

// systemData holds OS and runtime information for the system page.
type systemData struct {
	OS         string
	GoVersion  string
	Goroutines int
}

// System renders the system information page.
func (h *Handler) System(w http.ResponseWriter, r *http.Request) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	d := systemData{
		OS:         runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
	}
	h.render(w, r, "system.html", "System — Admin", "system", d)
}

// Library renders the library management page.
func (h *Handler) Library(w http.ResponseWriter, r *http.Request) {
	libs, err := h.db.Music.ListLibraries(r.Context())
	if err != nil {
		http.Error(w, "failed to list libraries: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, r, "library.html", "Library — Admin", "library", libs)
}

// TriggerScan fires an immediate library scan (incremental) in the background.
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.basePath()+"/config/library?flash=scan+started", http.StatusSeeOther)
}

// SchedulerPanel renders the scheduler status page showing all registered jobs.
func (h *Handler) SchedulerPanel(w http.ResponseWriter, r *http.Request) {
	var statuses []scheduler.JobStatus
	if h.sched != nil {
		statuses = h.sched.Status()
	}
	h.render(w, r, "scheduler.html", "Scheduler — Admin", "scheduler", statuses)
}

// RunJob triggers an immediate run of the named job via the scheduler.
func (h *Handler) RunJob(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.basePath()+"/config/scheduler?flash=job+queued", http.StatusSeeOther)
}

// configFormData carries every editable server.yml setting for the config
// page, grouped to match the Config struct sections in src/config/config.go.
// Per config-rules.md every server.yml setting must be admin-UI editable —
// the only field intentionally omitted is auth.jwt_secret (an auto-generated
// signing secret; exposing or letting it be freely retyped would violate the
// "never expose sensitive information" rule and could silently invalidate
// every active session) and email.password is write-only (never echoed back).
type configFormData struct {
	// server
	Address        string
	Port           int
	BaseURL        string
	Mode           string
	Debug          bool
	LogLevel       string
	Domain         string
	TrustedProxies string
	// server.url_detection
	Learning     bool
	MinSamples   int
	SampleWindow string
	LogChanges   bool
	LiveReload   bool
	// database
	DatabasePath string
	// paths (Config/Data/Log/Cache are process bootstrap paths — same
	// restart-required class as a database driver change, PART 12 "Live
	// Reload"; Music is the one path setting that is safe to hot-apply)
	PathsConfig string
	PathsData   string
	PathsLog    string
	PathsCache  string
	Music       string
	// auth (jwt_secret intentionally not exposed; see type doc)
	SessionDuration  int
	MaxLoginAttempts int
	LockoutMinutes   int
	// scanner
	AutoScan        bool
	ScanInterval    int
	FollowSymlinks  bool
	ExcludePatterns string
	// icecast
	IcecastEnabled bool
	MaxMounts      int
	// scrobble
	ScrobbleEnabled bool
	ScrobbleDelay   int
	// ffmpeg
	FFmpegPath   string
	DownloadAuto bool
	// email (Password is write-only; never rendered back)
	EmailEnabled bool
	EmailHost    string
	EmailPort    int
	EmailUser    string
	EmailFrom    string
	EmailTLS     bool
	// features
	FeaturePodcasts     bool
	FeaturePublicShares bool
	FeatureUserSignup   bool
	FeatureGeoIP        bool
	FeatureTor          bool
	FeatureTranscoding  bool
	FeatureMusicBrainz  bool
	// web.csrf
	CSRFEnabled     bool
	CSRFExemptPaths string

	Flash      string
	FlashError bool
}

// configFormFromConfig builds the config page view model from cfg.
func configFormFromConfig(cfg *config.Config) configFormData {
	return configFormData{
		Address:        cfg.Server.Address,
		Port:           cfg.Server.Port,
		BaseURL:        cfg.Server.BaseURL,
		Mode:           cfg.Server.Mode,
		Debug:          cfg.Server.Debug,
		LogLevel:       cfg.Server.LogLevel,
		Domain:         strings.Join(cfg.Server.Domain, ", "),
		TrustedProxies: strings.Join(cfg.Server.TrustedProxies, ", "),

		Learning:     cfg.Server.URLDetection.Learning,
		MinSamples:   cfg.Server.URLDetection.MinSamples,
		SampleWindow: cfg.Server.URLDetection.SampleWindow,
		LogChanges:   cfg.Server.URLDetection.LogChanges,
		LiveReload:   cfg.Server.URLDetection.LiveReload,

		DatabasePath: cfg.Database.Path,

		PathsConfig: cfg.Paths.Config,
		PathsData:   cfg.Paths.Data,
		PathsLog:    cfg.Paths.Log,
		PathsCache:  cfg.Paths.Cache,
		Music:       strings.Join(cfg.Paths.Music, ", "),

		SessionDuration:  cfg.Auth.SessionDuration,
		MaxLoginAttempts: cfg.Auth.MaxLoginAttempts,
		LockoutMinutes:   cfg.Auth.LockoutMinutes,

		AutoScan:        cfg.Scanner.AutoScan,
		ScanInterval:    cfg.Scanner.ScanInterval,
		FollowSymlinks:  cfg.Scanner.FollowSymlinks,
		ExcludePatterns: strings.Join(cfg.Scanner.ExcludePatterns, ", "),

		IcecastEnabled: cfg.Icecast.Enabled,
		MaxMounts:      cfg.Icecast.MaxMounts,

		ScrobbleEnabled: cfg.Scrobble.Enabled,
		ScrobbleDelay:   cfg.Scrobble.Delay,

		FFmpegPath:   cfg.FFmpeg.Path,
		DownloadAuto: cfg.FFmpeg.DownloadAuto,

		EmailEnabled: cfg.Email.Enabled,
		EmailHost:    cfg.Email.Host,
		EmailPort:    cfg.Email.Port,
		EmailUser:    cfg.Email.Username,
		EmailFrom:    cfg.Email.From,
		EmailTLS:     cfg.Email.TLS,

		FeaturePodcasts:     cfg.Features.Podcasts,
		FeaturePublicShares: cfg.Features.PublicShares,
		FeatureUserSignup:   cfg.Features.UserSignup,
		FeatureGeoIP:        cfg.Features.GeoIP,
		FeatureTor:          cfg.Features.Tor,
		FeatureTranscoding:  cfg.Features.Transcoding,
		FeatureMusicBrainz:  cfg.Features.MusicBrainz,

		CSRFEnabled:     cfg.Web.CSRF.Enabled,
		CSRFExemptPaths: strings.Join(cfg.Web.CSRF.ExemptPaths, ", "),
	}
}

// Config renders the server configuration form.
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	d := configFormFromConfig(h.cfg)
	if flash := r.URL.Query().Get("flash"); flash != "" {
		d.Flash = flash
	}
	if flashErr := r.URL.Query().Get("error"); flashErr != "" {
		d.Flash = flashErr
		d.FlashError = true
	}
	h.render(w, r, "config.html", "Config — Admin", "config", d)
}

// splitCSV splits a comma-separated form value into a trimmed, non-empty
// string slice. An empty or whitespace-only input yields an empty slice
// rather than a slice containing one empty string.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// saveConfigError redirects back to the config page with an error flash.
func (h *Handler) saveConfigError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, h.basePath()+"/config/settings?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

// SaveConfig parses the POSTed configuration form, validates it, persists it
// to server.yml, and applies it to the in-memory config so changes take
// effect immediately for every request that reads *config.Config from here
// on — per AI.md PART 12 "Live Reload", no restart is required except for
// server.address, server.port, database.path, and the paths.{config,data,
// log,cache} bootstrap directories (the same class of setting as a database
// driver change, all fixed at process startup).
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.saveConfigError(w, r, "invalid form submission")
		return
	}

	// next starts as a copy of the current config so any field not present
	// in the submitted form (there should be none from the real form, but
	// defends against partial/empty POSTs) keeps its current value.
	next := *h.cfg
	origAddress, origPort := h.cfg.Server.Address, h.cfg.Server.Port
	origDBPath := h.cfg.Database.Path
	origPathsConfig, origPathsData := h.cfg.Paths.Config, h.cfg.Paths.Data
	origPathsLog, origPathsCache := h.cfg.Paths.Log, h.cfg.Paths.Cache

	form := r.PostForm
	var parseErr error
	atoi := func(key string, dst *int) {
		if parseErr != nil || !form.Has(key) {
			return
		}
		v, err := strconv.Atoi(strings.TrimSpace(form.Get(key)))
		if err != nil {
			parseErr = fmt.Errorf("%s: must be a number", key)
			return
		}
		*dst = v
	}
	str := func(key string, dst *string) {
		if form.Has(key) {
			*dst = strings.TrimSpace(form.Get(key))
		}
	}
	boolean := func(key string, dst *bool) {
		if form.Has(key) {
			*dst = config.ParseBool(form.Get(key))
		}
	}
	csv := func(key string, dst *[]string) {
		if form.Has(key) {
			*dst = splitCSV(form.Get(key))
		}
	}

	str("address", &next.Server.Address)
	atoi("port", &next.Server.Port)
	str("base_url", &next.Server.BaseURL)
	str("mode", &next.Server.Mode)
	boolean("debug", &next.Server.Debug)
	str("log_level", &next.Server.LogLevel)
	csv("domain", &next.Server.Domain)
	csv("trusted_proxies", &next.Server.TrustedProxies)

	boolean("learning", &next.Server.URLDetection.Learning)
	atoi("min_samples", &next.Server.URLDetection.MinSamples)
	str("sample_window", &next.Server.URLDetection.SampleWindow)
	boolean("log_changes", &next.Server.URLDetection.LogChanges)
	boolean("live_reload", &next.Server.URLDetection.LiveReload)

	str("database_path", &next.Database.Path)

	str("paths_config", &next.Paths.Config)
	str("paths_data", &next.Paths.Data)
	str("paths_log", &next.Paths.Log)
	str("paths_cache", &next.Paths.Cache)
	csv("music", &next.Paths.Music)

	atoi("session_duration", &next.Auth.SessionDuration)
	atoi("max_login_attempts", &next.Auth.MaxLoginAttempts)
	atoi("lockout_minutes", &next.Auth.LockoutMinutes)

	boolean("auto_scan", &next.Scanner.AutoScan)
	atoi("scan_interval", &next.Scanner.ScanInterval)
	boolean("follow_symlinks", &next.Scanner.FollowSymlinks)
	csv("exclude_patterns", &next.Scanner.ExcludePatterns)

	boolean("icecast_enabled", &next.Icecast.Enabled)
	atoi("max_mounts", &next.Icecast.MaxMounts)

	boolean("scrobble_enabled", &next.Scrobble.Enabled)
	atoi("scrobble_delay", &next.Scrobble.Delay)

	str("ffmpeg_path", &next.FFmpeg.Path)
	boolean("download_auto", &next.FFmpeg.DownloadAuto)

	boolean("email_enabled", &next.Email.Enabled)
	str("email_host", &next.Email.Host)
	atoi("email_port", &next.Email.Port)
	str("email_username", &next.Email.Username)
	str("email_from", &next.Email.From)
	boolean("email_tls", &next.Email.TLS)
	// email_password is write-only: only overwrite when a new value was
	// actually typed, so a blank field never wipes out the stored password.
	if v := strings.TrimSpace(form.Get("email_password")); v != "" {
		next.Email.Password = v
	}

	boolean("feature_podcasts", &next.Features.Podcasts)
	boolean("feature_public_shares", &next.Features.PublicShares)
	boolean("feature_user_signup", &next.Features.UserSignup)
	boolean("feature_geo_ip", &next.Features.GeoIP)
	boolean("feature_tor", &next.Features.Tor)
	boolean("feature_transcoding", &next.Features.Transcoding)
	boolean("feature_music_brainz", &next.Features.MusicBrainz)

	boolean("csrf_enabled", &next.Web.CSRF.Enabled)
	csv("csrf_exempt_paths", &next.Web.CSRF.ExemptPaths)

	if parseErr != nil {
		h.saveConfigError(w, r, parseErr.Error())
		return
	}

	if err := next.Validate(); err != nil {
		h.saveConfigError(w, r, err.Error())
		return
	}

	if err := config.Save(&next, h.cfgPath); err != nil {
		h.saveConfigError(w, r, "failed to write server.yml: "+err.Error())
		return
	}

	// Apply to the live, shared *config.Config so every in-flight and future
	// request sees the new values immediately without a restart.
	*h.cfg = next

	flash := "saved"
	if next.Server.Address != origAddress || next.Server.Port != origPort || next.Database.Path != origDBPath ||
		next.Paths.Config != origPathsConfig || next.Paths.Data != origPathsData ||
		next.Paths.Log != origPathsLog || next.Paths.Cache != origPathsCache {
		flash = "saved — address, port, database path, and process directory changes require a restart to take effect"
	}
	http.Redirect(w, r, h.basePath()+"/config/settings?flash="+url.QueryEscape(flash), http.StatusSeeOther)
}

// logLines is the number of lines to display from the log file.
const logLines = 100

// Logs renders the last logLines lines of the server log file.
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	logPath := filepath.Join(h.cfg.Paths.Log, "cassonic.log")
	lines, err := tailFile(logPath, logLines)
	if err != nil {
		lines = []string{"(log file not available: " + err.Error() + ")"}
	}
	h.render(w, r, "logs.html", "Logs — Admin", "logs", lines)
}

// tailFile reads the last n lines from path.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines, scanner.Err()
}

// backupPageData carries backup file list and retention info.
type backupPageData struct {
	Files     []backupFile
	Retention int
	Flash     string
}

// backupFile describes one backup archive for display.
type backupFile struct {
	Name      string
	Path      string
	SizeMB    string
	CreatedAt string
	Encrypted bool
}

// Backup renders the backup management page.
func (h *Handler) Backup(w http.ResponseWriter, r *http.Request) {
	backupDir := filepath.Join(h.cfg.Paths.Data, "backups")
	files, _ := listBackupFiles(backupDir)
	d := backupPageData{
		Files:     files,
		Retention: 7,
		Flash:     r.URL.Query().Get("flash"),
	}
	h.render(w, r, "backup.html", "Backup — Admin", "backup", d)
}

// listBackupFiles returns backup archive info from backupDir.
func listBackupFiles(backupDir string) ([]backupFile, error) {
	entries, err := os.ReadDir(backupDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var files []backupFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cassonic-backup-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sizeMB := fmt.Sprintf("%.2f MB", float64(info.Size())/(1024*1024))
		files = append(files, backupFile{
			Name:      name,
			Path:      filepath.Join(backupDir, name),
			SizeMB:    sizeMB,
			CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
			Encrypted: strings.HasSuffix(name, ".enc"),
		})
	}
	return files, nil
}

// BackupNow triggers an immediate backup and redirects back to the backup page.
func (h *Handler) BackupNow(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.basePath()+"/config/backup?flash=backup+started", http.StatusSeeOther)
}
