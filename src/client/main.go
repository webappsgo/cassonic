// cassonic-cli - companion CLI for the cassonic music streaming server
// See AI.md PART 33 for the full client specification.
package main

import (
	"fmt"
	"os"
	"strconv"
)

// Build info — set via -ldflags at build time.
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", err)
		cfg = defaultConfig()
	}

	// Global flags.
	var (
		flagServer    = ""
		flagToken     = ""
		flagTokenFile = ""
		flagDebug     = cfg.Debug
		flagColor     = cfg.Color
		flagJSON      = false
		flagFormat    = ""
	)

	args := os.Args[1:]
	args = parseGlobalFlags(args, &flagServer, &flagToken, &flagTokenFile, &flagDebug, &flagColor, &flagJSON, &flagFormat)

	// Handle --help / --version before anything else.
	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}
	switch args[0] {
	case "--help", "-h":
		printHelp()
		os.Exit(0)
	case "--version", "-v":
		printVersion()
		os.Exit(0)
	}

	// CASSONIC_COLOR env var applies only when --color was not given
	// explicitly, keeping the precedence flag > env > config file > default.
	if flagColor == cfg.Color {
		if v := os.Getenv("CASSONIC_COLOR"); v != "" {
			flagColor = v
		}
	}
	initColor(flagColor)

	// Output format precedence: --format flag > CASSONIC_FORMAT env >
	// cli.yml output.format > legacy --json flag > compiled default (table).
	format := flagFormat
	if format == "" {
		if v := os.Getenv("CASSONIC_FORMAT"); v != "" {
			format = v
		} else if cfg.Format != "" {
			format = cfg.Format
		} else if flagJSON {
			format = "json"
		} else {
			format = "table"
		}
	}
	if !validFormat(format) {
		printError(fmt.Sprintf("invalid --format %q; must be one of: table, json, plain", format))
		os.Exit(1)
	}

	serverURL := cfg.Server.URL
	if flagServer != "" {
		serverURL = flagServer
	} else if v := os.Getenv("CASSONIC_SERVER"); v != "" {
		serverURL = v
	}
	if serverURL == "" {
		serverURL = "http://localhost:4533"
	}

	token := resolveToken(flagToken, flagTokenFile, cfg)
	client := newClient(serverURL, token, flagDebug)

	cmd := args[0]
	rest := args[1:]

	if err := dispatch(client, cfg, cmd, rest, format); err != nil {
		printError(err.Error())
		os.Exit(1)
	}
}

// parseGlobalFlags strips recognized global flags from args and sets the pointed-to values.
// Returns the remaining args (the command and its arguments).
func parseGlobalFlags(args []string, server, token, tokenFile *string, debug *bool, color *string, wantJSON *bool, format *string) []string {
	remaining := args[:0]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--debug":
			*debug = true
		case arg == "--json":
			*wantJSON = true
		case arg == "--format" && i+1 < len(args):
			i++
			*format = args[i]
		case len(arg) > 9 && arg[:9] == "--format=":
			*format = arg[9:]
		case arg == "--server" && i+1 < len(args):
			i++
			*server = args[i]
		case len(arg) > 9 && arg[:9] == "--server=":
			*server = arg[9:]
		case arg == "--token" && i+1 < len(args):
			i++
			*token = args[i]
		case len(arg) > 8 && arg[:8] == "--token=":
			*token = arg[8:]
		case arg == "--token-file" && i+1 < len(args):
			i++
			*tokenFile = args[i]
		case len(arg) > 13 && arg[:13] == "--token-file=":
			*tokenFile = arg[13:]
		case arg == "--color" && i+1 < len(args):
			i++
			*color = args[i]
		case len(arg) > 8 && arg[:8] == "--color=":
			*color = arg[8:]
		default:
			remaining = append(remaining, arg)
		}
	}
	return remaining
}

// validFormat reports whether f is a recognized --format value.
func validFormat(f string) bool {
	switch f {
	case "table", "json", "plain":
		return true
	default:
		return false
	}
}

// dispatch routes the command to the appropriate handler.
func dispatch(c *Client, cfg CLIConfig, cmd string, args []string, format string) error {
	switch cmd {
	case "login":
		return cmdLogin(c, cfg)

	case "logout":
		return cmdLogout(c)

	case "status":
		return cmdStatus(c, format)

	case "scan":
		full := false
		libID := "default"
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--full":
				full = true
			default:
				libID = args[i]
			}
		}
		return cmdScan(c, libID, full, format)

	case "scan-status":
		return cmdScanStatus(c, format)

	case "artists":
		page, limit := parsePagination(args)
		return cmdArtists(c, page, limit, format)

	case "albums":
		page, limit := parsePagination(args)
		return cmdAlbums(c, page, limit, format)

	case "songs":
		page, limit := parsePagination(args)
		return cmdSongs(c, page, limit, format)

	case "genres":
		return cmdGenres(c, format)

	case "search":
		if len(args) == 0 {
			return fmt.Errorf("usage: cassonic-cli search {query}")
		}
		return cmdSearch(c, args[0], format)

	case "playlists":
		return cmdPlaylists(c, format)

	case "playlist":
		if len(args) == 0 {
			return fmt.Errorf("usage: cassonic-cli playlist {id}")
		}
		return cmdPlaylist(c, args[0], format)

	case "playlist-create":
		if len(args) == 0 {
			return fmt.Errorf("usage: cassonic-cli playlist-create {name}")
		}
		return cmdPlaylistCreate(c, args[0], format)

	case "playlist-add":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli playlist-add {playlist-id} {song-id}")
		}
		return cmdPlaylistAdd(c, args[0], args[1])

	case "playlist-remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli playlist-remove {playlist-id} {song-id}")
		}
		return cmdPlaylistRemove(c, args[0], args[1])

	case "tags":
		if len(args) == 0 {
			return fmt.Errorf("usage: cassonic-cli tags {song-id}")
		}
		return cmdTags(c, args[0], format)

	case "tags-set":
		if len(args) == 0 {
			return fmt.Errorf("usage: cassonic-cli tags-set {song-id} [--title T] [--artist A] [--album B] [--year Y] [--track N] [--genre G]")
		}
		songID := args[0]
		fields := parseTagSetFlags(args[1:])
		return cmdTagsSet(c, songID, fields, format)

	case "icecast":
		return dispatchIcecast(c, args, format)

	case "users":
		return dispatchUsers(c, args, format)

	case "--help", "-h":
		printHelp()
		return nil

	case "--version", "-v":
		printVersion()
		return nil

	default:
		return fmt.Errorf("unknown command: %s\nRun 'cassonic-cli --help' for usage", cmd)
	}
}

// dispatchIcecast handles the icecast sub-commands.
func dispatchIcecast(c *Client, args []string, format string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cassonic-cli icecast {list|start|stop} [mount-id]")
	}
	switch args[0] {
	case "list":
		return cmdIcecastList(c, format)
	case "start":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli icecast start {mount-id}")
		}
		return cmdIcecastStart(c, args[1])
	case "stop":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli icecast stop {mount-id}")
		}
		return cmdIcecastStop(c, args[1])
	default:
		return fmt.Errorf("unknown icecast sub-command: %s", args[0])
	}
}

// dispatchUsers handles the users sub-commands.
func dispatchUsers(c *Client, args []string, format string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cassonic-cli users {list|create|delete} [args]")
	}
	switch args[0] {
	case "list":
		return cmdUsersList(c, format)
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli users create {username} [--admin]")
		}
		username := args[1]
		admin := false
		for _, a := range args[2:] {
			if a == "--admin" {
				admin = true
			}
		}
		return cmdUsersCreate(c, username, admin, format)
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: cassonic-cli users delete {username}")
		}
		return cmdUsersDelete(c, args[1])
	default:
		return fmt.Errorf("unknown users sub-command: %s", args[0])
	}
}

// parsePagination extracts --page and --limit flags from args.
func parsePagination(args []string) (page, limit int) {
	page = 1
	limit = 50
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--page" && i+1 < len(args):
			i++
			if v, err := strconv.Atoi(args[i]); err == nil {
				page = v
			}
		case len(args[i]) > 7 && args[i][:7] == "--page=":
			if v, err := strconv.Atoi(args[i][7:]); err == nil {
				page = v
			}
		case args[i] == "--limit" && i+1 < len(args):
			i++
			if v, err := strconv.Atoi(args[i]); err == nil {
				limit = v
			}
		case len(args[i]) > 8 && args[i][:8] == "--limit=":
			if v, err := strconv.Atoi(args[i][8:]); err == nil {
				limit = v
			}
		}
	}
	return page, limit
}

// parseTagSetFlags extracts tag field flags from args.
func parseTagSetFlags(args []string) TagSetFields {
	var f TagSetFields
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--title" && i+1 < len(args):
			i++
			f.Title = args[i]
		case args[i] == "--artist" && i+1 < len(args):
			i++
			f.Artist = args[i]
		case args[i] == "--album" && i+1 < len(args):
			i++
			f.Album = args[i]
		case args[i] == "--year" && i+1 < len(args):
			i++
			f.Year = args[i]
		case args[i] == "--track" && i+1 < len(args):
			i++
			f.Track = args[i]
		case args[i] == "--genre" && i+1 < len(args):
			i++
			f.Genre = args[i]
		}
	}
	return f
}

// printVersion prints the binary version string.
func printVersion() {
	fmt.Printf("cassonic-cli %s (commit: %s, built: %s)\n", Version, CommitID, BuildDate)
	if OfficialSite != "" {
		fmt.Printf("Official site: %s\n", OfficialSite)
	}
}

// printHelp prints the full usage message.
func printHelp() {
	fmt.Print(`cassonic-cli - CLI companion for the cassonic music streaming server

Usage:
  cassonic-cli [global-flags] <command> [args]

Global flags:
  --server {url}        Server URL (overrides config)
  --token {token}       API token
  --token-file {file}   Read token from file
  --debug               Debug HTTP requests
  --color {auto|yes|no} Color output (default: auto)
  --format {table|json|plain} Output format (default: table)
  --json                Shorthand for --format json
  --help, -h            Show help
  --version, -v         Show version

Commands:
  login                 Authenticate and save token
  logout                Revoke token and clear saved token
  status                Show server health and version

  Library:
  scan [library-id] [--full]     Trigger library scan
  scan-status                    Show current scan status

  Browse:
  artists [--page N] [--limit N]   List artists
  albums  [--page N] [--limit N]   List albums
  songs   [--page N] [--limit N]   List songs
  genres                           List genres
  search {query}                   Search everything

  Playlists:
  playlists                              List playlists
  playlist {id}                          Show playlist detail
  playlist-create {name}                 Create playlist
  playlist-add {playlist-id} {song-id}   Add song to playlist
  playlist-remove {playlist-id} {song-id} Remove song from playlist

  Tags:
  tags {song-id}                         Get song tags
  tags-set {song-id} [--title T] [--artist A] [--album B]
           [--year Y] [--track N] [--genre G]
                                         Set tag fields

  Icecast:
  icecast list                     List Icecast mounts
  icecast start {mount-id}         Start a mount
  icecast stop {mount-id}          Stop a mount

  Admin:
  users list                       List users (admin only)
  users create {username} [--admin] Create user
  users delete {username}          Delete user

Config file: ~/.config/local/cassonic/cli.yml
Token file:  ~/.config/local/cassonic/token
`)
}
