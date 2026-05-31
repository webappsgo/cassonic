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
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	handle := windows.Handle(os.Stdin.Fd())
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return readLine()
	}
	// Disable echo and line input so we read raw characters.
	newMode := oldMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return readLine()
	}
	defer func() {
		windows.SetConsoleMode(handle, oldMode)
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
