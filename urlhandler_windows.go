package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
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

// openBrowser asks the shell to open the link in the default browser. ShellExecute is
// used directly so the long login URL is never re-parsed by rundll32 or cmd.
func openBrowser(link string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	target, err := syscall.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	// ShellExecuteW returns a value above 32 on success
	ret, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(
		0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	if ret <= 32 {
		return fmt.Errorf("ShellExecute failed with code %d", ret)
	}
	return nil
}

func focusTerminal() {}
