//go:build windows

// terminal_windows.go - Windows terminal utilities (password input, isatty)
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// isTerminal reports whether the given file descriptor is a terminal on Windows.
func isTerminal(fd int) bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(uintptr(fd)), &mode)
	return err == nil
}

// readPassword reads a password from stdin without echoing characters on Windows.
// r must be the same *bufio.Reader used for any preceding stdin prompts so
// no buffered-ahead input is lost between reads.
func readPassword(prompt string, r *bufio.Reader) (string, error) {
	fmt.Print(prompt)
	handle := windows.Handle(os.Stdin.Fd())
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return readLine(r)
	}
	// Disable echo and line input so we read raw characters.
	newMode := oldMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return readLine(r)
	}
	defer func() {
		windows.SetConsoleMode(handle, oldMode)
		fmt.Println()
	}()
	return readLine(r)
}

// readLine reads a single line from r, stripping the trailing newline.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
