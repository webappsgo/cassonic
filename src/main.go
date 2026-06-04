// cassonic - self-hosted music streaming server
// See AI.md for implementation rules and specification.
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/local/cassonic/src/config"
	"github.com/local/cassonic/src/paths"
	"github.com/local/cassonic/src/server"
	"github.com/local/cassonic/src/server/service"
	svcbackup "github.com/local/cassonic/src/server/service/backup"
	"github.com/local/cassonic/src/server/service/email"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/service/geoip"
	"github.com/local/cassonic/src/server/service/install"
	"github.com/local/cassonic/src/server/service/musicbrainz"
	"github.com/local/cassonic/src/server/service/podcast"
	"github.com/local/cassonic/src/server/service/scheduler"
	"github.com/local/cassonic/src/server/service/scrobble"
	"github.com/local/cassonic/src/server/service/tags"
	svctor "github.com/local/cassonic/src/server/service/tor"
	svcupdate "github.com/local/cassonic/src/server/service/update"
	"github.com/local/cassonic/src/server/ssl"
	"github.com/local/cassonic/src/server/store"
)

// Build info — set via -ldflags at build time.
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

// activeLang holds the language code selected via --lang for i18n use.
var activeLang = "en"

// ansiEnabled controls whether ANSI colour codes are emitted.
var ansiEnabled = true

func main() {
	var (
		flagHelp        = flag.Bool("help", false, "Show help")
		flagVersion     = flag.Bool("version", false, "Show version")
		flagConfig      = flag.String("config", "", "Config directory")
		flagData        = flag.String("data", "", "Data directory")
		flagLog         = flag.String("log", "", "Log directory")
		flagAddress     = flag.String("address", "", "Listen address")
		flagPort        = flag.Int("port", 0, "Listen port")
		flagBaseURL     = flag.String("baseurl", "", "Base URL path prefix")
		flagMode        = flag.String("mode", "", "Server mode: production|development")
		flagDebug       = flag.Bool("debug", false, "Enable debug output")
		flagScan        = flag.Bool("scan", false, "Run library scan and exit")
		flagStatus      = flag.Bool("status", false, "Show server status and exit")
		flagPID         = flag.String("pid", "", "Write PID to file")
		flagInstall     = flag.Bool("install", false, "Install cassonic as a system service and exit")
		flagUninstall   = flag.Bool("uninstall", false, "Remove the cassonic system service and exit")
		flagBackupDir   = flag.String("backup-dir", "", "Directory for automatic backups (optional)")
		flagTorKey      = flag.String("tor-key", "", "Path to persist the Tor hidden service ed25519 key (optional; enables Tor)")
		flagService     = flag.String("service", "", "Service management: start|restart|stop|reload|--install|--uninstall|--disable|--help")
		flagDaemon      = flag.Bool("daemon", false, "Fork cassonic to run in the background")
		flagMaintenance = flag.String("maintenance", "", "Maintenance operation: backup|restore|update|mode|setup|--help")
		flagUpdate      = flag.String("update", "", "Check or apply updates: check|yes|branch=stable|branch=beta|branch=daily")
		flagLang        = flag.String("lang", "en", "UI language: en|es|fr|de|zh|ja|ar")
		flagColor       = flag.String("color", "auto", "ANSI colour output: auto|yes|no")
		flagTLS         = flag.Bool("tls", false, "Enable TLS (HTTPS)")
		flagTLSDomain   = flag.String("tls-domain", "", "Domain name for Let's Encrypt certificate")
		flagTLSEmail    = flag.String("tls-email", "", "Email address for Let's Encrypt ACME registration")
		flagTLSCert     = flag.String("tls-cert", "", "Path to local TLS certificate file (PEM)")
		flagTLSKey      = flag.String("tls-key", "", "Path to local TLS private key file (PEM)")
	)

	// Register short aliases: only -h and -v are permitted per spec.
	flag.BoolVar(flagHelp, "h", false, "Show help")
	flag.BoolVar(flagVersion, "v", false, "Show version")

	flag.CommandLine.Usage = printHelp
	flag.Parse()

	// Resolve colour preference before any output.
	resolveColor(*flagColor)

	if *flagHelp {
		printHelp()
		os.Exit(0)
	}
	if *flagVersion {
		printVersion()
		os.Exit(0)
	}

	// Language selection — validate against supported codes.
	setLang(*flagLang)

	// --service subcommand routing.
	if *flagService != "" {
		handleServiceCmd(*flagService)
		return
	}

	// --install / --uninstall convenience aliases remain available.
	if *flagInstall {
		runInstall()
		return
	}

	if *flagUninstall {
		runUninstall()
		return
	}

	// --daemon: re-exec the current binary without --daemon in a detached process.
	if *flagDaemon {
		runDaemon()
		return
	}

	// --update subcommand.
	if *flagUpdate != "" {
		handleUpdateCmd(*flagUpdate)
		return
	}

	overrides := map[string]string{}
	if *flagConfig != "" {
		overrides["config"] = *flagConfig
	}
	if *flagData != "" {
		overrides["data"] = *flagData
	}
	if *flagLog != "" {
		overrides["log"] = *flagLog
	}

	detectedPaths := paths.Detect(overrides)
	if err := paths.EnsureAll(detectedPaths); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: failed to create directories: %v\n", err)
		os.Exit(1)
	}

	cfgPath := filepath.Join(detectedPaths.Config, "server.yml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Defaults()
		if saveErr := config.Save(cfg, cfgPath); saveErr != nil {
			fmt.Fprintf(os.Stderr, "cassonic: warning: could not save default config: %v\n", saveErr)
		}
	}

	if *flagAddress != "" {
		cfg.Server.Address = *flagAddress
	}
	if *flagPort != 0 {
		cfg.Server.Port = *flagPort
	}
	if *flagBaseURL != "" {
		cfg.Server.BaseURL = *flagBaseURL
	}
	if *flagMode != "" {
		cfg.Server.Mode = *flagMode
	}
	if *flagDebug {
		cfg.Server.Debug = true
	}

	if cfg.Paths.Config == "" {
		cfg.Paths.Config = detectedPaths.Config
	}
	if cfg.Paths.Data == "" {
		cfg.Paths.Data = detectedPaths.Data
	}
	if cfg.Paths.Log == "" {
		cfg.Paths.Log = detectedPaths.Log
	}
	if cfg.Paths.Cache == "" {
		cfg.Paths.Cache = detectedPaths.Cache
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = filepath.Join(detectedPaths.Data, "server.db")
	}

	// First-run SMTP autodetect: if smtp host is empty, try to find a working server.
	if cfg.Email.Host == "" {
		if host, port, ok := email.AutoDetectSMTP(); ok {
			cfg.Email.Host = host
			cfg.Email.Port = port
			log.Printf("cassonic: SMTP autodetected at %s:%d", host, port)
			_ = config.Save(cfg, cfgPath)
		}
	}

	// --maintenance subcommand (needs cfg + detectedPaths resolved).
	if *flagMaintenance != "" {
		handleMaintenanceCmd(*flagMaintenance, cfg, cfgPath, *flagBackupDir, detectedPaths.Data)
		return
	}

	if *flagPID != "" {
		if err := writePID(*flagPID); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: warning: could not write PID file: %v\n", err)
		} else {
			defer removePID(*flagPID)
		}
	}

	server.Version = Version
	server.CommitID = CommitID
	server.BuildDate = BuildDate

	db, err := store.Open(detectedPaths.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close(db)

	ff, ffErr := ffmpeg.New(cfg.FFmpeg.Path, detectedPaths.Data, cfg.FFmpeg.DownloadAuto)
	if ffErr != nil {
		log.Printf("cassonic: warning: ffmpeg not available: %v", ffErr)
		ff = nil
	}

	tagReader := tags.New()

	scanLogger := log.New(os.Stdout, "[scanner] ", log.LstdFlags)
	scanner := service.NewScanner(db.Music, tagReader, scanLogger)

	thumbDir := filepath.Join(detectedPaths.Cache, "thumbs")
	coverArt := service.NewCoverArtService(db.Music, thumbDir)

	if *flagScan {
		fmt.Println("cassonic: running library scan...")
		if err := scanner.Scan(context.Background(), service.ScanModeFull); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: scan failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("cassonic: scan complete")
		os.Exit(0)
	}

	if *flagStatus {
		printStatus(db)
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	podcastLogger := log.New(os.Stdout, "[podcast] ", log.LstdFlags)
	podcastSvc := podcast.NewService(db, detectedPaths.Data, podcastLogger)

	scrobbleLogger := log.New(os.Stdout, "[scrobble] ", log.LstdFlags)
	scrobbleSvc := scrobble.NewService(db, scrobbleLogger)

	mbClient := musicbrainz.NewClient(Version)

	schedLogger := log.New(os.Stdout, "[scheduler] ", log.LstdFlags)
	sched := scheduler.New(schedLogger)
	sched.Register(scheduler.LibraryScanJob(scanner))
	sched.Register(scheduler.PodcastRefreshJob(podcastSvc))
	sched.Register(scheduler.ScrobbleRetryJob(scrobbleSvc))
	sched.Register(scheduler.MusicBrainzLookupJob(mbClient, db.Music))
	sched.Register(scheduler.CoverArtRefreshJob(coverArt))
	sched.Register(scheduler.SSLRenewalJob(filepath.Join(detectedPaths.Config, "ssl", "letsencrypt"), schedLogger))
	sched.Register(scheduler.GeoIPUpdateJob(filepath.Join(detectedPaths.Data, "security", "geoip"), schedLogger))
	sched.Register(scheduler.BlocklistUpdateJob(detectedPaths.Data, schedLogger))
	sched.Register(scheduler.CVEUpdateJob(detectedPaths.Data, schedLogger))
	sched.Register(scheduler.SessionCleanupJob(db.Users))
	sched.Register(scheduler.TokenCleanupJob(db.Users))
	sched.Register(scheduler.LogRotationJob(detectedPaths.Log, schedLogger))
	sched.Register(scheduler.HealthCheckSelfJob(cfg.Server.Port, schedLogger))
	sched.Register(scheduler.TorHealthJob(schedLogger))
	sched.Register(scheduler.ClusterHeartbeatJob(db, schedLogger))
	sched.Start(ctx)

	srv := server.New(cfg, db, scanner, coverArt, ff, tagReader)

	geoipDB, _ := geoip.OpenOptional(filepath.Join(detectedPaths.Data, "security", "geoip", "country.mmdb"))
	if geoipDB != nil {
		srv.WithGeoIP(geoipDB, nil, nil)
	}

	if *flagBackupDir != "" {
		backupLogger := log.New(os.Stdout, "[backup] ", log.LstdFlags)
		backupCfg := svcbackup.Config{
			Dir:       *flagBackupDir,
			Retention: 30,
		}
		backupSvc := svcbackup.New(backupCfg, detectedPaths.Data, backupLogger)
		srv.WithBackupService(backupSvc)
	}

	// Wire SSL/TLS when requested.
	if *flagTLS {
		sslCfg := ssl.Config{
			Enabled:   true,
			Domain:    *flagTLSDomain,
			Email:     *flagTLSEmail,
			LocalCert: *flagTLSCert,
			LocalKey:  *flagTLSKey,
			CertDir:   filepath.Join(detectedPaths.Config, "ssl", "letsencrypt"),
		}
		sslMgr := ssl.New(sslCfg)
		if sslMgr != nil {
			srv.WithSSL(sslMgr)
		}
	}

	// Auto-enable Tor when the tor binary is present and no key path was specified.
	if *flagTorKey == "" {
		if _, lookErr := exec.LookPath("tor"); lookErr == nil {
			defaultTorKey := filepath.Join(detectedPaths.Data, "tor", "hidden_service.key")
			*flagTorKey = defaultTorKey
			log.Printf("cassonic: tor binary found, auto-enabling hidden service")
		}
	}

	if *flagTorKey != "" {
		torLogger := log.New(os.Stdout, "[tor] ", log.LstdFlags)
		torSvc := svctor.New(*flagTorKey, torLogger)
		onionAddr, torErr := torSvc.Start(ctx, cfg.Server.Port)
		if torErr != nil {
			log.Printf("cassonic: warning: Tor hidden service failed to start: %v", torErr)
		} else {
			fmt.Printf("cassonic: Tor hidden service: http://%s\n", onionAddr)
		}
		defer func() {
			if stopErr := torSvc.Stop(); stopErr != nil {
				log.Printf("cassonic: tor stop: %v", stopErr)
			}
		}()
	}

	// Print language in startup banner when non-default.
	if activeLang != "en" {
		fmt.Printf("cassonic: language: %s\n", activeLang)
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: server error: %v\n", err)
		os.Exit(1)
	}
}

// setLang validates and stores the requested language code.
func setLang(lang string) {
	supported := map[string]bool{
		"en": true, "es": true, "fr": true, "de": true,
		"zh": true, "ja": true, "ar": true,
	}
	if supported[lang] {
		activeLang = lang
	} else {
		fmt.Fprintf(os.Stderr, "cassonic: unsupported language %q; using en\n", lang)
		activeLang = "en"
	}
}

// resolveColor applies the --color flag and the NO_COLOR environment variable
// to the ansiEnabled package variable.
func resolveColor(mode string) {
	if os.Getenv("NO_COLOR") != "" || mode == "no" {
		ansiEnabled = false
		return
	}
	if mode == "yes" {
		ansiEnabled = true
		return
	}
	// "auto": enable only when stdout is a terminal (fd 1).
	fi, err := os.Stdout.Stat()
	if err != nil {
		ansiEnabled = false
		return
	}
	ansiEnabled = (fi.Mode() & os.ModeCharDevice) != 0
}

// handleServiceCmd processes the --service subcommand value.
func handleServiceCmd(cmd string) {
	switch cmd {
	case "start":
		fmt.Println("cassonic: use your init system to start cassonic (e.g. systemctl start cassonic)")
	case "restart":
		if err := install.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: service restart (stop) failed: %v\n", err)
			os.Exit(1)
		}
		runInstall()
	case "stop":
		if err := install.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: service stop failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("cassonic: service stopped and removed")
	case "reload":
		fmt.Println("cassonic: send SIGHUP to the running cassonic process to reload configuration")
	case "--install":
		runInstall()
	case "--uninstall":
		runUninstall()
	case "--disable":
		if err := install.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: service disable failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("cassonic: service disabled and removed")
	case "--help":
		fmt.Print(`cassonic --service usage:

  --service start         Print guidance on starting the service via the init system
  --service restart       Reinstall the service (stop + install)
  --service stop          Stop and remove the service
  --service reload        Print guidance on reloading the service configuration
  --service --install     Install cassonic as a system service
  --service --uninstall   Remove the cassonic system service
  --service --disable     Disable and remove the cassonic system service
  --service --help        Show this help
`)
	default:
		fmt.Fprintf(os.Stderr, "cassonic: unknown --service value %q; use --service --help\n", cmd)
		os.Exit(1)
	}
}

// runInstall installs the system service using the install package.
func runInstall() {
	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: cannot determine binary path: %v\n", err)
		os.Exit(1)
	}
	cfg := install.Config{
		BinaryPath:  selfPath,
		ConfigDir:   "/etc/local/cassonic",
		DataDir:     "/var/lib/local/cassonic",
		LogDir:      "/var/log/local/cassonic",
		User:        "cassonic",
		Group:       "cassonic",
		Description: "cassonic - self-hosted music streaming server",
	}
	if err := install.Install(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("cassonic: service installed")
}

// runUninstall removes the system service using the install package.
func runUninstall() {
	if err := install.Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: uninstall failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("cassonic: service removed")
}

// runDaemon re-executes the current binary in the background without --daemon.
// On Windows the process runs in the foreground (service manager controls it).
func runDaemon() {
	if runtime.GOOS == "windows" {
		fmt.Println("cassonic: --daemon is not required on Windows; use the service manager")
		return
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: cannot determine binary path: %v\n", err)
		os.Exit(1)
	}

	// Build argument list without --daemon to avoid infinite re-exec.
	args := make([]string, 0, len(os.Args))
	for _, a := range os.Args[1:] {
		if a != "--daemon" && a != "-daemon" {
			args = append(args, a)
		}
	}

	cmd := exec.Command(self, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: failed to start background process: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("cassonic: running in background (pid %d)\n", cmd.Process.Pid)
}

// handleUpdateCmd processes the --update flag value.
func handleUpdateCmd(val string) {
	logger := log.New(os.Stdout, "[update] ", log.LstdFlags)
	checker := svcupdate.New(Version, logger)
	ctx := context.Background()

	// Parse optional branch= prefix and apply it to the checker.
	action := val
	if strings.HasPrefix(val, "branch=") {
		branch := strings.TrimPrefix(val, "branch=")
		checker = checker.WithBranch(branch)
		fmt.Printf("cassonic: update branch set to %q (use --update check or --update yes to act)\n", checker.Branch())
		return
	}

	switch action {
	case "check":
		rel, err := checker.CheckLatest(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: update check failed: %v\n", err)
			os.Exit(1)
		}
		if checker.IsNewer(rel) {
			fmt.Printf("cassonic: new version available: %s (current: %s)\n", rel.Version, Version)
			if rel.DownloadURL != "" {
				fmt.Printf("cassonic: download: %s\n", rel.DownloadURL)
			}
		} else {
			fmt.Printf("cassonic: already up to date (%s)\n", Version)
		}
	case "yes":
		rel, err := checker.CheckLatest(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: update check failed: %v\n", err)
			os.Exit(1)
		}
		if !checker.IsNewer(rel) {
			fmt.Printf("cassonic: already up to date (%s)\n", Version)
			return
		}
		fmt.Printf("cassonic: updating to %s...\n", rel.Version)
		if rel.DownloadURL == "" {
			fmt.Fprintln(os.Stderr, "cassonic: no download URL in release; cannot auto-update")
			os.Exit(1)
		}
		if err := applyUpdate(rel); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: update failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cassonic: updated to %s; please restart the service\n", rel.Version)
	default:
		fmt.Fprintf(os.Stderr, "cassonic: unknown --update value %q; use check|yes|branch=<stable|beta|daily>\n", val)
		os.Exit(1)
	}
}

// applyUpdate downloads the release binary, verifies its SHA-256 checksum,
// and performs an atomic replacement of the running binary.
func applyUpdate(rel *svcupdate.Release) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}

	// Download to a temp file adjacent to the binary.
	tmpPath := self + ".update.tmp"
	if err := downloadFile(rel.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	// Verify SHA-256 when a checksum is provided.
	if rel.Checksum != "" {
		if err := verifySHA256(tmpPath, rel.Checksum); err != nil {
			return fmt.Errorf("checksum mismatch: %w", err)
		}
	}

	// Make the downloaded binary executable.
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if runtime.GOOS == "windows" {
		// On Windows rename the running binary then place the new one.
		oldPath := self + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(self, oldPath); err != nil {
			return fmt.Errorf("rename running binary: %w", err)
		}
		if err := os.Rename(tmpPath, self); err != nil {
			_ = os.Rename(oldPath, self)
			return fmt.Errorf("place new binary: %w", err)
		}
		return nil
	}

	// Unix: atomic replace via os.Rename.
	return os.Rename(tmpPath, self)
}

// handleMaintenanceCmd processes the --maintenance flag value.
func handleMaintenanceCmd(cmd string, cfg *config.Config, cfgPath, backupDir, dataDir string) {
	switch cmd {
	case "backup":
		if backupDir == "" {
			fmt.Fprintln(os.Stderr, "cassonic: --backup-dir is required for --maintenance backup")
			os.Exit(1)
		}
		logger := log.New(os.Stdout, "[backup] ", log.LstdFlags)
		svc := svcbackup.New(svcbackup.Config{Dir: backupDir, Retention: 30}, dataDir, logger)
		path, err := svc.Backup(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: backup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cassonic: backup written to %s\n", path)
	case "update":
		fmt.Println("cassonic: use --update check or --update yes to manage updates")
	case "setup":
		fmt.Println("cassonic: current configuration paths:")
		fmt.Printf("  config: %s\n", cfg.Paths.Config)
		fmt.Printf("  data:   %s\n", cfg.Paths.Data)
		fmt.Printf("  log:    %s\n", cfg.Paths.Log)
		fmt.Printf("  config file: %s\n", cfgPath)
	case "--help":
		fmt.Print(`cassonic --maintenance usage:

  --maintenance backup    Create a backup archive (requires --backup-dir)
  --maintenance restore   Restore from a backup file
  --maintenance update    Print guidance on using --update
  --maintenance mode      Change server.mode in server.yml
  --maintenance setup     Print current configuration paths
  --maintenance --help    Show this help
`)
	default:
		// Handle "restore {file}" and "mode {value}" passed as single token.
		if strings.HasPrefix(cmd, "restore") {
			parts := strings.SplitN(cmd, " ", 2)
			if len(parts) < 2 || parts[1] == "" {
				fmt.Fprintln(os.Stderr, "cassonic: --maintenance restore requires a file path, e.g. --maintenance 'restore /path/to/backup.tar.gz'")
				os.Exit(1)
			}
			if backupDir == "" {
				fmt.Fprintln(os.Stderr, "cassonic: --backup-dir is required for --maintenance restore")
				os.Exit(1)
			}
			logger := log.New(os.Stdout, "[backup] ", log.LstdFlags)
			svc := svcbackup.New(svcbackup.Config{Dir: backupDir, Retention: 30}, dataDir, logger)
			if err := svc.Restore(context.Background(), parts[1]); err != nil {
				fmt.Fprintf(os.Stderr, "cassonic: restore failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("cassonic: restore complete")
			return
		}
		if strings.HasPrefix(cmd, "mode") {
			parts := strings.SplitN(cmd, " ", 2)
			if len(parts) < 2 || parts[1] == "" {
				fmt.Fprintln(os.Stderr, "cassonic: --maintenance mode requires a value: production|development")
				os.Exit(1)
			}
			newMode := strings.TrimSpace(parts[1])
			if newMode != "production" && newMode != "development" {
				fmt.Fprintf(os.Stderr, "cassonic: invalid mode %q; must be production or development\n", newMode)
				os.Exit(1)
			}
			cfg.Server.Mode = newMode
			if err := config.Save(cfg, cfgPath); err != nil {
				fmt.Fprintf(os.Stderr, "cassonic: failed to save config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("cassonic: server.mode set to %q in %s\n", newMode, cfgPath)
			return
		}
		fmt.Fprintf(os.Stderr, "cassonic: unknown --maintenance value %q; use --maintenance --help\n", cmd)
		os.Exit(1)
	}
}

// downloadFile fetches url and writes it to dest.
func downloadFile(url, dest string) error {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// verifySHA256 checks that the file at path has the given hex-encoded SHA-256 digest.
func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	if got != strings.ToLower(want) {
		return fmt.Errorf("got %s, want %s", got, want)
	}
	return nil
}

func writePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

func removePID(path string) {
	_ = os.Remove(path)
}

func printStatus(db *store.DB) {
	fmt.Printf("cassonic status:\n")
	status, err := db.Music.GetLastScanStatus(context.Background())
	if err != nil || status == nil {
		fmt.Printf("  Last scan: never\n")
		return
	}
	fmt.Printf("  Last scan: %s (%s)\n", status.StartedAt.Format(time.RFC3339), status.Status)
	fmt.Printf("  Songs: %d scanned, %d added, %d updated\n",
		status.ScannedFiles, status.AddedFiles, status.UpdatedFiles)
}

func printVersion() {
	fmt.Printf("cassonic %s (commit: %s, built: %s)\n", Version, CommitID, BuildDate)
	if OfficialSite != "" {
		fmt.Printf("Official site: %s\n", OfficialSite)
	}
}

func printHelp() {
	fmt.Print(`cassonic - self-hosted music streaming server

Usage:
  cassonic [flags]

Flags:
  --help                             Show help
  --version / -v                     Show version
  --mode {production|development}    Server mode (default: production)
  --config {dir}                     Config directory
  --data {dir}                       Data directory
  --log {dir}                        Log directory
  --address {addr}                   Listen address (default: all interfaces)
  --port {port}                      Listen port (default: 4533)
  --baseurl {path}                   Base URL path prefix
  --debug                            Enable debug output
  --scan                             Run library scan and exit
  --status                           Show server status and exit
  --pid {file}                       Write PID to file
  --install                          Install cassonic as a system service and exit
  --uninstall                        Remove the cassonic system service and exit
  --backup-dir {dir}                 Directory for backup archives (optional)
  --tor-key {file}                   Path to persist Tor ed25519 key; enables Tor hidden service (optional)
  --service {cmd}                    Manage system service: start|restart|stop|reload|--install|--uninstall|--disable|--help
  --daemon                           Fork cassonic to run in the background
  --maintenance {cmd}                Maintenance operation: backup|restore|update|mode|setup|--help
  --update {check|yes}               Check for or apply updates; prefix with branch= to select a release channel
  --lang {code}                      UI language: en|es|fr|de|zh|ja|ar (default: en)
  --color {auto|yes|no}              ANSI colour output (default: auto)
  --tls                              Enable TLS (HTTPS)
  --tls-domain {domain}              Domain name for Let's Encrypt certificate
  --tls-email {email}                Email address for Let's Encrypt ACME registration
  --tls-cert {file}                  Path to local TLS certificate (PEM)
  --tls-key {file}                   Path to local TLS private key (PEM)
`)
}
