// output.go - tabwriter and ANSI color helpers
package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// ANSI escape sequences — no external library.
const (
	ansiReset    = "\033[0m"
	ansiBoldCyan = "\033[1;36m"
	ansiGreen    = "\033[32m"
	ansiRed      = "\033[31m"
	ansiBold     = "\033[1m"
)

// colorEnabled tracks whether ANSI output is active.
var colorEnabled bool

// initColor sets colorEnabled based on the --color flag and environment.
// Checks NO_COLOR env var, --color flag, and whether stdout is a terminal.
func initColor(colorFlag string) {
	if os.Getenv("NO_COLOR") != "" {
		colorEnabled = false
		return
	}
	switch colorFlag {
	case "yes":
		colorEnabled = true
	case "no":
		colorEnabled = false
	default:
		// auto: enable only when stdout is a real terminal.
		colorEnabled = isTerminal(int(os.Stdout.Fd()))
	}
}

// colorize wraps text with an ANSI code when color is enabled.
func colorize(code, text string) string {
	if !colorEnabled {
		return text
	}
	return code + text + ansiReset
}

// printHeading prints a bold-cyan section heading.
func printHeading(s string) {
	fmt.Println(colorize(ansiBoldCyan, s))
}

// printSuccess prints a green success message.
func printSuccess(s string) {
	fmt.Println(colorize(ansiGreen, s))
}

// printError prints a red error message to stderr.
func printError(s string) {
	fmt.Fprintln(os.Stderr, colorize(ansiRed, "error: "+s))
}

// newTabWriter creates a tabwriter suitable for CLI table output.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}
