// cassonic-cli - companion CLI for cassonic server
// See AI.md PART 33 for client specification.
package main

import (
	"fmt"
	"os"
)

// Build info - set via -ldflags at build time
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

func main() {
	// Minimal stub - see AI.md PART 33 for full CLI implementation
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "--version", "-v":
			printVersion()
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "cassonic-cli: client not yet implemented — see AI.md PART 33\n")
	os.Exit(1)
}

func printVersion() {
	fmt.Printf("cassonic-cli %s (commit: %s, built: %s)\n", Version, CommitID, BuildDate)
	if OfficialSite != "" {
		fmt.Printf("Official site: %s\n", OfficialSite)
	}
}

func printHelp() {
	fmt.Print(`cassonic-cli - CLI companion for cassonic server

Usage:
  cassonic-cli [flags] [command]

Flags:
  --help, -h          Show help
  --version, -v       Show version
  --server {url}      Server URL (default from site.txt or --server flag required)
  --debug             Enable debug output
  --color {auto|yes|no}

See AI.md PART 33 for complete client specification.
`)
}
