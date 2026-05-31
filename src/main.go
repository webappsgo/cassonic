// cassonic - self-hosted music streaming server
// See AI.md for implementation rules and specification.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/local/cassonic/src/config"
	"github.com/local/cassonic/src/paths"
	"github.com/local/cassonic/src/server"
	"github.com/local/cassonic/src/server/service"
	"github.com/local/cassonic/src/server/service/ffmpeg"
	"github.com/local/cassonic/src/server/service/install"
	"github.com/local/cassonic/src/server/service/musicbrainz"
	"github.com/local/cassonic/src/server/service/podcast"
	"github.com/local/cassonic/src/server/service/scheduler"
	"github.com/local/cassonic/src/server/service/scrobble"
	"github.com/local/cassonic/src/server/service/tags"
	"github.com/local/cassonic/src/server/store"
)

// Build info — set via -ldflags at build time.
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

func main() {
	var (
		flagHelp      = flag.Bool("help", false, "Show help")
		flagVersion   = flag.Bool("version", false, "Show version")
		flagConfig    = flag.String("config", "", "Config directory")
		flagData      = flag.String("data", "", "Data directory")
		flagLog       = flag.String("log", "", "Log directory")
		flagAddress   = flag.String("address", "", "Listen address")
		flagPort      = flag.Int("port", 0, "Listen port")
		flagBaseURL   = flag.String("baseurl", "", "Base URL path prefix")
		flagMode      = flag.String("mode", "", "Server mode: production|development")
		flagDebug     = flag.Bool("debug", false, "Enable debug output")
		flagScan      = flag.Bool("scan", false, "Run library scan and exit")
		flagStatus    = flag.Bool("status", false, "Show server status and exit")
		flagPID       = flag.String("pid", "", "Write PID to file")
		flagInstall   = flag.Bool("install", false, "Install cassonic as a system service and exit")
		flagUninstall = flag.Bool("uninstall", false, "Remove the cassonic system service and exit")
	)

	flag.CommandLine.Usage = printHelp
	flag.Parse()

	if *flagHelp {
		printHelp()
		os.Exit(0)
	}
	if *flagVersion {
		printVersion()
		os.Exit(0)
	}

	if *flagInstall {
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
		os.Exit(0)
	}

	if *flagUninstall {
		if err := install.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "cassonic: uninstall failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("cassonic: service removed")
		os.Exit(0)
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
	sched.Start(ctx)

	srv := server.New(cfg, db, scanner, coverArt, ff, tagReader)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cassonic: server error: %v\n", err)
		os.Exit(1)
	}
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
  --help                          Show help
  --version                       Show version
  --mode {production|development} Server mode (default: production)
  --config {dir}                  Config directory
  --data {dir}                    Data directory
  --log {dir}                     Log directory
  --address {addr}                Listen address (default: all interfaces)
  --port {port}                   Listen port (default: 4533)
  --baseurl {path}                Base URL path prefix
  --debug                         Enable debug output
  --scan                          Run library scan and exit
  --status                        Show server status and exit
  --pid {file}                    Write PID to file
  --install                       Install cassonic as a system service and exit
  --uninstall                     Remove the cassonic system service and exit
`)
}
