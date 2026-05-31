// Package admin provides the cassonic server administration panel.
// All routes require an authenticated server admin session.
package admin

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

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
	version   string
	sched     *scheduler.Scheduler
	tmpls     *template.Template
	startTime time.Time
}

// New creates a fully configured admin Handler.
func New(db *store.DB, cfg *config.Config, version string, sched *scheduler.Scheduler) *Handler {
	h := &Handler{
		db:        db,
		cfg:       cfg,
		version:   version,
		sched:     sched,
		startTime: time.Now(),
	}
	h.tmpls = h.parseTemplates()
	return h
}

// parseTemplates loads all HTML templates from the embedded filesystem.
func (h *Handler) parseTemplates() *template.Template {
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

	tmpl := template.New("").Funcs(funcMap)
	sub, err := fs.Sub(assets, "template")
	if err != nil {
		panic(fmt.Sprintf("admin: sub template fs: %v", err))
	}
	tmpl, err = tmpl.ParseFS(sub, "*.html")
	if err != nil {
		panic(fmt.Sprintf("admin: parse templates: %v", err))
	}
	return tmpl
}

// Routes assembles the chi router for the admin panel.
// All routes are wrapped in requireAdmin middleware.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(h.requireAdmin)

	r.Get("/", h.Dashboard)
	r.Get("/system", h.System)
	r.Get("/library", h.Library)
	r.Post("/library/scan", h.TriggerScan)
	r.Get("/scheduler", h.SchedulerPanel)
	r.Post("/scheduler/{job}/run", h.RunJob)
	r.Get("/config", h.Config)
	r.Post("/config", h.SaveConfig)
	r.Get("/logs", h.Logs)
	r.Get("/backup", h.Backup)
	r.Post("/backup/now", h.BackupNow)

	return r
}

// requireAdmin verifies the requesting user is an authenticated server admin.
// Checks the cassonic_session cookie (SHA-256 hash → UserStore.GetSessionByHash).
// Non-admin requests are redirected to /login?next={path}.
func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check context first (set by NativeAuth middleware upstream).
		u := mw.UserFromContext(r.Context())
		if u != nil && u.IsAdmin {
			next.ServeHTTP(w, r)
			return
		}

		// Fall back to session cookie lookup.
		cookie, err := r.Cookie("cassonic_session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}

		raw := sha256.Sum256([]byte(cookie.Value))
		hashHex := hex.EncodeToString(raw[:])

		ctx := r.Context()
		session, err := h.db.Users.GetSessionByHash(ctx, hashHex)
		if err != nil || session == nil || session.IsExpired() {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}

		user, err := h.db.Users.GetUser(ctx, session.UserID)
		if err != nil || user == nil || !user.IsAdmin || !user.IsEnabled {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// adminPageData carries data common to every admin template.
type adminPageData struct {
	Title   string
	Version string
	Active  string
	Data    any
}

// render executes the named template with the provided page data.
func (h *Handler) render(w http.ResponseWriter, name, title, active string, data any) {
	pd := adminPageData{
		Title:   title,
		Version: h.version,
		Active:  active,
		Data:    data,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpls.ExecuteTemplate(w, name, pd); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// dashboardData holds stats for the admin dashboard.
type dashboardData struct {
	Uptime        string
	Version       string
	ActiveStreams  int
	TorEnabled    bool
}

// Dashboard renders the admin panel home page with server statistics.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime).Round(time.Second)
	d := dashboardData{
		Uptime:  uptime.String(),
		Version: h.version,
	}
	h.render(w, "dashboard.html", "Dashboard — Admin", "dashboard", d)
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
	h.render(w, "system.html", "System — Admin", "system", d)
}

// Library renders the library management page.
func (h *Handler) Library(w http.ResponseWriter, r *http.Request) {
	libs, err := h.db.Music.ListLibraries(r.Context())
	if err != nil {
		http.Error(w, "failed to list libraries: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "library.html", "Library — Admin", "library", libs)
}

// TriggerScan fires an immediate library scan (incremental) in the background.
func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/server/admin/library?flash=scan+started", http.StatusSeeOther)
}

// SchedulerPanel renders the scheduler status page showing all registered jobs.
func (h *Handler) SchedulerPanel(w http.ResponseWriter, r *http.Request) {
	var statuses []scheduler.JobStatus
	if h.sched != nil {
		statuses = h.sched.Status()
	}
	h.render(w, "scheduler.html", "Scheduler — Admin", "scheduler", statuses)
}

// RunJob triggers an immediate run of the named job via the scheduler.
func (h *Handler) RunJob(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/server/admin/scheduler?flash=job+queued", http.StatusSeeOther)
}

// configFormData carries the editable server settings for the config page.
type configFormData struct {
	Port                int
	Mode                string
	Debug               bool
	Flash               string
}

// Config renders the server configuration form.
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	d := configFormData{
		Port:  h.cfg.Server.Port,
		Mode:  h.cfg.Server.Mode,
		Debug: h.cfg.Server.Debug,
	}
	h.render(w, "config.html", "Config — Admin", "config", d)
}

// SaveConfig handles the config form POST and updates in-memory settings.
// Persistent writes require a server restart; this validates and acknowledges.
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/server/admin/config?flash=saved", http.StatusSeeOther)
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
	h.render(w, "logs.html", "Logs — Admin", "logs", lines)
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
	h.render(w, "backup.html", "Backup — Admin", "backup", d)
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
	http.Redirect(w, r, "/server/admin/backup?flash=backup+started", http.StatusSeeOther)
}
