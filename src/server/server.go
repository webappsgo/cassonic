// Package server wires all handlers, middleware, and services into a running
// HTTP server with graceful shutdown.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/local/cassonic/src/config"
	handlerapi "github.com/local/cassonic/src/server/handler/api"
	"github.com/local/cassonic/src/server/handler/ampache"
	"github.com/local/cassonic/src/server/handler/subsonic"
	"github.com/local/cassonic/src/server/handler/web"
	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/metrics"
	"github.com/local/cassonic/src/server/service"
	svcbackup "github.com/local/cassonic/src/server/service/backup"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/service/geoip"
	"github.com/local/cassonic/src/server/service/scheduler"
	"github.com/local/cassonic/src/server/service/tags"
	"github.com/local/cassonic/src/server/ssl"
	"github.com/local/cassonic/src/server/store"
	handleradmin "github.com/local/cassonic/src/server/handler/admin"
)

// Version, CommitID, and BuildDate are set via -ldflags at build time.
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

// Server is the cassonic HTTP server.
type Server struct {
	cfg         *config.Config
	db          *store.DB
	scanner     *service.Scanner
	coverArt    *service.CoverArtService
	ffmpeg      *ffmpeg.Manager
	tagReader   *tags.Reader
	ampSessions *mw.AmpacheSessionStore
	http        *http.Server
	backupSvc   *svcbackup.Service
	sslMgr      *ssl.Manager

	// rate limiters per API layer
	nativeRL   *mw.RateLimiter
	subsonicRL *mw.RateLimiter
	ampacheRL  *mw.RateLimiter
	loginRL    *mw.RateLimiter

	// IP filter
	ipFilter *mw.IPFilter

	// GeoIP filtering
	geoipDB        *geoip.DB
	denyCountries  []string
	allowCountries []string

	// MetricsToken, when non-empty, requires a matching Bearer token to access /metrics.
	MetricsToken string

	// sched is the built-in scheduler; exposed to the admin panel for status display.
	sched *scheduler.Scheduler
}

// New creates and fully configures the HTTP server. It does not begin listening.
func New(
	cfg *config.Config,
	db *store.DB,
	scanner *service.Scanner,
	coverArt *service.CoverArtService,
	ff *ffmpeg.Manager,
	tagReader *tags.Reader,
) *Server {
	s := &Server{
		cfg:       cfg,
		db:        db,
		scanner:   scanner,
		coverArt:  coverArt,
		ffmpeg:    ff,
		tagReader: tagReader,
	}

	s.nativeRL = mw.NewRateLimiter(100, 200)
	s.subsonicRL = mw.NewRateLimiter(60, 120)
	s.ampacheRL = mw.NewRateLimiter(60, 120)
	s.loginRL = mw.NewRateLimiter(5, 5)

	s.ipFilter = mw.NewIPFilter(nil, nil)

	s.ampSessions = mw.NewAmpacheSessionStore(24 * time.Hour)

	router := s.buildRouter()

	s.http = &http.Server{
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// buildRouter assembles the complete chi router with all middleware and routes.
func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()

	// Global middleware stack — order is enforced by spec (PART 13).
	r.Use(mw.RequestID())
	r.Use(s.ipFilter.Middleware())
	r.Use(mw.GeoIPFilter(s.geoipDB, s.denyCountries, s.allowCountries))
	r.Use(mw.Logger(os.Stdout))
	r.Use(mw.Cors())
	r.Use(mw.SecurityHeaders(s.cfg.Server.Mode == "production"))
	r.Use(s.metricsMiddleware())

	// Suppress the chi default middleware's own request-id header to avoid
	// duplication with our own RequestID middleware.
	_ = chimw.RequestID

	// Public health and version endpoints — no auth required.
	r.Get("/server/healthz", s.healthzHTML())
	r.Get("/health", s.healthzJSON())
	r.Get("/api/v1/health", s.healthzJSON())
	r.Get("/version", s.versionJSON())
	r.Get("/api/version", s.versionJSON())
	r.Get("/api/v1/version", s.versionJSON())
	r.Get("/api/v1/autodiscover", s.autodiscoverJSON())

	// Swagger UI — served at /api/docs/* and mirrored at /swagger/*.
	r.Get("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/docs/", http.StatusMovedPermanently)
	})
	r.Get("/api/docs/", s.swaggerUI())
	r.Get("/api/docs/*", s.swaggerUI())
	r.Get("/swagger/", s.swaggerUI())
	r.Get("/swagger/*", s.swaggerUI())

	// OpenAPI spec served from an embedded JSON constant.
	r.Get("/api/v1/openapi.json", s.openAPISpec())

	// Prometheus metrics — internal only (guarded by IP filter at infra level).
	// Optional Bearer token auth when MetricsToken is configured.
	r.Handle("/metrics", s.metricsHandler())

	// Native REST API — auth is optional at the middleware level; individual
	// routes enforce RequireAuth / RequireAdmin via their own With() calls.
	r.Group(func(r chi.Router) {
		r.Use(s.nativeRL.Middleware("native"))
		r.Use(mw.NativeAuth(s.db.Users))
		// Login endpoint gets tighter rate limiting; mount before the group handler.
		r.With(s.loginRL.Middleware("login")).Post("/api/v1/auth/login", s.nativeHandler().Login)
		r.Mount("/", s.nativeHandler().Routes())
	})

	// Subsonic REST API.
	r.Group(func(r chi.Router) {
		r.Use(s.subsonicRL.Middleware("subsonic"))
		r.Use(mw.SubsonicAuth(s.db.Users, s.getSubsonicPassword))
		r.Mount("/", s.subsonicHandler().Routes())
	})

	// Ampache API — auth middleware is applied inside the handler's own Routes().
	r.Group(func(r chi.Router) {
		r.Use(s.ampacheRL.Middleware("ampache"))
		r.Mount("/", s.ampacheHandler().Routes())
	})

	// Admin panel — mounted before WebUI catch-all.
	r.Mount("/server/admin", s.adminHandler().Routes())

	// WebUI — catch-all last; includes its own embedded /static/* handler.
	r.Mount("/", s.webHandler().Routes())

	return r
}

// Start begins listening on the configured address and port. It blocks until the
// server shuts down (SIGINT or SIGTERM received) then drains with a 30-second
// timeout. When an SSL manager is attached, it serves HTTPS and runs an
// HTTP→HTTPS redirect server on port 80 in a background goroutine.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Address, s.cfg.Server.Port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	if s.sslMgr != nil {
		tlsCfg := s.sslMgr.TLSConfig()
		s.http.TLSConfig = tlsCfg

		// Start a plain-HTTP listener on port 80 to redirect to HTTPS.
		redirectAddr := fmt.Sprintf("%s:80", s.cfg.Server.Address)
		redirectSrv := &http.Server{
			Addr:         redirectAddr,
			Handler:      s.sslMgr.HTTPHandler(nil),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		go func() {
			if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "cassonic: http redirect server: %v\n", err)
			}
		}()

		fmt.Printf("cassonic listening on https://%s\n", addr)

		certFile, keyFile := s.sslMgr.CertFiles()
		go func() {
			if serveErr := s.http.ListenAndServeTLS(certFile, keyFile); serveErr != nil && serveErr != http.ErrServerClosed {
				errCh <- serveErr
			}
		}()
	} else {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("server: listen %s: %w", addr, err)
		}
		fmt.Printf("cassonic listening on http://%s\n", ln.Addr())
		go func() {
			if serveErr := s.http.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
				errCh <- serveErr
			}
		}()
	}

	select {
	case sig := <-sigCh:
		fmt.Printf("cassonic: received %s, shutting down...\n", sig)
	case err := <-errCh:
		return fmt.Errorf("server: serve: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("server: shutdown: %w", err)
	}

	fmt.Println("cassonic: shutdown complete")
	return nil
}

// getSubsonicPassword retrieves a deterministic Subsonic token for a user.
// Token auth requires a separate subsonic_token field on the user model; until
// that is added, this returns false so that Subsonic clients fall back to
// plaintext password verification via Argon2id.
func (s *Server) getSubsonicPassword(_ context.Context, _ string) (string, bool) {
	return "", false
}

// WithGeoIP attaches a GeoIP database and country filter lists to the server.
// deny and allow are ISO 3166-1 alpha-2 codes. When allow is non-empty it takes precedence.
func (s *Server) WithGeoIP(db *geoip.DB, deny, allow []string) *Server {
	s.geoipDB = db
	s.denyCountries = deny
	s.allowCountries = allow
	s.http.Handler = s.buildRouter()
	return s
}

// WithBackupService attaches an optional backup service to the server.
func (s *Server) WithBackupService(svc *svcbackup.Service) *Server {
	s.backupSvc = svc
	return s
}

// WithSSL attaches an SSL/TLS manager to the server. When set, Start()
// serves HTTPS and redirects plain-HTTP connections to HTTPS on port 80.
func (s *Server) WithSSL(m *ssl.Manager) *Server {
	s.sslMgr = m
	return s
}

// WithScheduler attaches the built-in scheduler so the admin panel can display job status.
func (s *Server) WithScheduler(sc *scheduler.Scheduler) *Server {
	s.sched = sc
	return s
}

// nativeHandler constructs the native API handler.
func (s *Server) nativeHandler() *handlerapi.Handler {
	h := handlerapi.NewHandler(s.db, s.scanner, s.coverArt, s.ffmpeg, s.tagReader)
	if s.backupSvc != nil {
		h.WithBackupService(s.backupSvc)
	}
	return h
}

// subsonicHandler constructs the Subsonic API handler.
func (s *Server) subsonicHandler() *subsonic.Handler {
	return subsonic.NewHandler(s.db, s.scanner, s.coverArt, s.ffmpeg, s.getSubsonicPassword)
}

// ampacheHandler constructs the Ampache API handler.
func (s *Server) ampacheHandler() *ampache.Handler {
	return ampache.NewHandler(s.db, s.ampSessions, s.scanner, s.coverArt, s.getSubsonicPassword)
}

// webHandler constructs the WebUI handler.
func (s *Server) webHandler() *web.Handler {
	return web.NewHandlerWithConfig(s.db, s.cfg, Version)
}

// adminHandler constructs the admin panel handler.
func (s *Server) adminHandler() *handleradmin.Handler {
	return handleradmin.New(s.db, s.cfg, Version, s.sched)
}

// healthzHTML returns an HTTP handler that writes a plain-text health page.
func (s *Server) healthzHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>cassonic health</title></head>`+
			`<body><p>OK — cassonic %s</p></body></html>`, Version)
	}
}

// healthzJSON returns an HTTP handler that writes a JSON health response.
func (s *Server) healthzJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": Version,
		})
	}
}

// swaggerUI returns an HTTP handler that serves the self-contained Swagger UI page.
func (s *Server) swaggerUI() http.HandlerFunc {
	const page = `<!DOCTYPE html>
<html>
<head>
  <title>cassonic API docs</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
SwaggerUIBundle({
  url: "/api/v1/openapi.json",
  dom_id: '#swagger-ui',
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
  layout: "BaseLayout",
  deepLinking: true
});
</script>
</body>
</html>`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, page)
	}
}

// openAPISpec returns an HTTP handler that serves the cassonic OpenAPI 3.0 specification.
func (s *Server) openAPISpec() http.HandlerFunc {
	const spec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "cassonic API",
    "description": "cassonic self-hosted music streaming server — native REST API.",
    "version": "1"
  },
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer"
      }
    }
  },
  "security": [{"BearerAuth": []}],
  "paths": {
    "/api/v1/health": {
      "get": {
        "summary": "Health check",
        "security": [],
        "responses": {"200": {"description": "Server is healthy"}}
      }
    },
    "/api/v1/version": {
      "get": {
        "summary": "Server version",
        "security": [],
        "responses": {"200": {"description": "Version info"}}
      }
    },
    "/api/v1/auth/login": {
      "post": {
        "summary": "Authenticate and obtain a session token",
        "security": [],
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object", "properties": {"username": {"type": "string"}, "password": {"type": "string"}}}}}
        },
        "responses": {"200": {"description": "Login successful, returns token"}}
      }
    },
    "/api/v1/artists": {
      "get": {
        "summary": "List artists",
        "responses": {"200": {"description": "Paginated list of artists"}}
      }
    },
    "/api/v1/artists/{id}": {
      "get": {
        "summary": "Get artist by ID",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "Artist object"}}
      }
    },
    "/api/v1/albums": {
      "get": {
        "summary": "List albums",
        "responses": {"200": {"description": "Paginated list of albums"}}
      }
    },
    "/api/v1/albums/{id}": {
      "get": {
        "summary": "Get album by ID",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "Album object"}}
      }
    },
    "/api/v1/songs": {
      "get": {
        "summary": "List songs",
        "responses": {"200": {"description": "Paginated list of songs"}}
      }
    },
    "/api/v1/songs/{id}/stream": {
      "get": {
        "summary": "Stream a song",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "Audio stream"}}
      }
    },
    "/api/v1/playlists": {
      "get": {
        "summary": "List playlists",
        "responses": {"200": {"description": "Paginated list of playlists"}}
      },
      "post": {
        "summary": "Create a playlist",
        "responses": {"201": {"description": "Created playlist"}}
      }
    },
    "/api/v1/search": {
      "get": {
        "summary": "Search artists, albums, and songs",
        "parameters": [{"name": "q", "in": "query", "required": true, "schema": {"type": "string"}}],
        "responses": {"200": {"description": "Search results"}}
      }
    },
    "/api/v1/libraries/{id}/scan": {
      "post": {
        "summary": "Trigger a library scan",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"202": {"description": "Scan started"}}
      }
    },
    "/api/v1/songs/{id}/tags": {
      "get": {
        "summary": "Read tags for a song",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "Tag map"}}
      },
      "patch": {
        "summary": "Update tags for a song",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"description": "Updated tag map"}}
      }
    },
    "/api/v1/admin/backup": {
      "post": {
        "summary": "Trigger a backup (admin only)",
        "responses": {"201": {"description": "Backup path and size"}}
      }
    },
    "/api/v1/admin/backups": {
      "get": {
        "summary": "List available backups (admin only)",
        "responses": {"200": {"description": "List of backup info objects"}}
      }
    },
    "/api/v1/admin/restore": {
      "post": {
        "summary": "Restore from a backup (admin only)",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object", "properties": {"path": {"type": "string"}}}}}
        },
        "responses": {"200": {"description": "Restore status"}}
      }
    }
  }
}`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, spec)
	}
}

// autodiscoverJSON returns an HTTP handler that advertises the server's
// capability endpoints for client auto-configuration.
func (s *Server) autodiscoverJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := s.cfg.Server.BaseURL
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"server":       "cassonic",
			"version":      Version,
			"api_version":  "v1",
			"base_url":     baseURL,
			"features":     []string{"subsonic", "ampache", "icecast", "podcasts", "scrobbling", "tor"},
			"subsonic_url": "/rest",
			"ampache_url":  "/server",
			"api_url":      "/api/v1",
			"docs_url":     "/swagger/",
			"metrics_url":  "/metrics",
		})
	}
}

// versionJSON returns an HTTP handler that writes version information as JSON.
func (s *Server) versionJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":   Version,
			"commit":    CommitID,
			"buildDate": BuildDate,
		})
	}
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware returns middleware that records cassonic_http_requests_total
// for every request, labelled by method, path, and response status code.
func (s *Server) metricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			metrics.HTTPRequests.WithLabelValues(
				r.Method,
				r.URL.Path,
				strconv.Itoa(rw.status),
			).Inc()
		})
	}
}

// metricsHandler returns an HTTP handler for /metrics with optional Bearer token auth.
// When MetricsToken is empty the metrics are accessible without authentication.
func (s *Server) metricsHandler() http.Handler {
	base := promhttp.Handler()
	if s.MetricsToken == "" {
		return base
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != s.MetricsToken {
			w.Header().Set("WWW-Authenticate", `Bearer realm="cassonic-metrics"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		base.ServeHTTP(w, r)
	})
}
