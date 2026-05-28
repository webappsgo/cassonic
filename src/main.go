// cassonic - main entry point
// See AI.md for implementation rules and specification.
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
	// Minimal stub - see AI.md PART 7, 8 for full CLI implementation
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

	fmt.Fprintf(os.Stderr, "cassonic: server not yet implemented — see AI.md PART 8\n")
	os.Exit(1)
}

func printVersion() {
	fmt.Printf("cassonic %s (commit: %s, built: %s)\n", Version, CommitID, BuildDate)
	if OfficialSite != "" {
		fmt.Printf("Official site: %s\n", OfficialSite)
	}
}

func printHelp() {
	fmt.Print(`cassonic - self-hosted server application

Usage:
  cassonic [flags]

Flags:
  --help, -h                    Show help
  --version, -v                 Show version
  --mode {production|development}
  --config {config_dir}
  --data {data_dir}
  --log {log_dir}
  --pid {pid_file}
  --address {listen}
  --port {port}
  --baseurl {path}
  --debug
  --status
  --service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
  --daemon
  --maintenance {backup,restore,update,mode,setup,--help} [optional-file-or-setting]
  --update [check|yes|branch {stable|beta|daily}]

See AI.md for complete specification.
`)
}
