//go:build darwin || freebsd || openbsd || netbsd || dragonfly

// terminal_bsd.go - BSD/Darwin terminal utilities (password input, isatty)
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether the given file descriptor is a terminal.
func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	return err == nil
}

// readPassword reads a password from stdin without echoing characters.
// r must be the same *bufio.Reader used for any preceding stdin prompts so
// no buffered-ahead input is lost between reads.
func readPassword(prompt string, r *bufio.Reader) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		// Not a real terminal — fall back to plain line read.
		return readLine(r)
	}
	newState := *oldState
	// Clear ECHO flag so characters are not displayed.
	newState.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &newState); err != nil {
		return readLine(r)
	}
	defer func() {
		// Always restore terminal state on return.
		unix.IoctlSetTermios(fd, unix.TIOCSETA, oldState)
		fmt.Println()
	}()
	return readLine(r)
}

// readLine reads a single line from r, stripping the trailing newline.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
