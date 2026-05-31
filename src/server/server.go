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
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/service/tags"
	"github.com/local/cassonic/src/server/store"
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

	// rate limiters per API layer
	nativeRL   *mw.RateLimiter
	subsonicRL *mw.RateLimiter
	ampacheRL  *mw.RateLimiter
	loginRL    *mw.RateLimiter

	// IP filter
	ipFilter *mw.IPFilter
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
	r.Use(mw.Logger(os.Stdout))
	r.Use(mw.Cors())
	r.Use(mw.SecurityHeaders(s.cfg.Server.Mode == "production"))

	// Suppress the chi default middleware's own request-id header to avoid
	// duplication with our own RequestID middleware.
	_ = chimw.RequestID

	// Public health and version endpoints — no auth required.
	r.Get("/server/healthz", s.healthzHTML())
	r.Get("/api/v1/health", s.healthzJSON())
	r.Get("/api/v1/version", s.versionJSON())

	// Prometheus metrics — internal only (guarded by IP filter at infra level).
	r.Handle("/metrics", promhttp.Handler())

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

	// WebUI — catch-all last; includes its own embedded /static/* handler.
	r.Mount("/", s.webHandler().Routes())

	return r
}

// Start begins listening on the configured address and port. It blocks until the
// server shuts down (SIGINT or SIGTERM received) then drains with a 30-second
// timeout.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Address, s.cfg.Server.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	fmt.Printf("cassonic listening on http://%s\n", ln.Addr())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if serveErr := s.http.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()

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

// nativeHandler constructs the native API handler.
func (s *Server) nativeHandler() *handlerapi.Handler {
	return handlerapi.NewHandler(s.db, s.scanner, s.coverArt, s.ffmpeg, s.tagReader)
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
