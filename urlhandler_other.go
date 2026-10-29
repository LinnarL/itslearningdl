//go:build !darwin && !windows

package main

import (
	"errors"
	"os/exec"
)

func installURLHandler(callback string) error {
	return errors.New("automatic login redirect is only supported on macOS")
}

func openBrowser(link string) error {
	return exec.Command("xdg-open", link).Run()
}

func focusTerminal() {}
