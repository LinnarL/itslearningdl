package main

import (
	"fmt"
	"os"
	"os/exec"
)

const urlScheme = "itsl-itslearning"

// installURLHandler registers this executable as the handler for itsl-itslearning://
// links (per user, no admin rights needed). The browser then starts a second copy with
// --handle-url, which forwards the link to the local callback server and exits.
func installURLHandler(callback string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	base := `HKCU\Software\Classes\` + urlScheme
	for _, args := range [][]string{
		{"add", base, "/ve", "/d", "URL:itslearning login", "/f"},
		{"add", base, "/v", "URL Protocol", "/d", "", "/f"},
		{"add", base + `\shell\open\command`, "/ve", "/d", fmt.Sprintf(`"%s" --handle-url "%%1"`, exe), "/f"},
	} {
		if out, err := exec.Command("reg", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("reg %v: %v: %s", args[0], err, out)
		}
	}
	return nil
}

func openBrowser(link string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Run()
}

func focusTerminal() {}
