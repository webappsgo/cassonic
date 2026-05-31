//go:build !windows

// terminal_unix.go - Unix terminal utilities (password input, isatty)
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
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

// readPassword reads a password from stdin without echoing characters.
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		// Not a real terminal — fall back to plain line read.
		return readLine()
	}
	newState := *oldState
	// Clear ECHO flag so characters are not displayed.
	newState.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return readLine()
	}
	defer func() {
		// Always restore terminal state on return.
		unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
		fmt.Println()
	}()
	return readLine()
}

// readLine reads a single line from stdin, stripping the trailing newline.
func readLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
