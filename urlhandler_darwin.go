package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

// installURLHandler builds a tiny background app that receives itsl-itslearning:// links
// from the browser and forwards them to the local callback server
func installURLHandler(callback string) error {
	support, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	app := filepath.Join(support, "itslearningdl", "Itslearning Login.app")
	script := fmt.Sprintf(`on open location theURL
	do shell script "curl -s -m 5 --data-urlencode " & quoted form of ("url=" & theURL) & " %s"
end open location
`, callback)

	os.RemoveAll(app)
	if err := os.MkdirAll(filepath.Dir(app), 0755); err != nil {
		return err
	}
	src := filepath.Join(filepath.Dir(app), "handler.applescript")
	if err := os.WriteFile(src, []byte(script), 0644); err != nil {
		return err
	}
	if out, err := exec.Command("osacompile", "-o", app, src).CombinedOutput(); err != nil {
		return fmt.Errorf("osacompile: %v: %s", err, out)
	}

	plist := filepath.Join(app, "Contents", "Info.plist")
	for _, cmd := range []string{
		"Delete :CFBundleIdentifier",
		"Add :CFBundleIdentifier string io.github.minie4.itslearningdl.login",
		"Add :LSUIElement bool true",
		"Add :CFBundleURLTypes array",
		"Add :CFBundleURLTypes:0 dict",
		"Add :CFBundleURLTypes:0:CFBundleURLName string itslearning login",
		"Add :CFBundleURLTypes:0:CFBundleURLSchemes array",
		"Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string itsl-itslearning",
	} {
		exec.Command("/usr/libexec/PlistBuddy", "-c", cmd, plist).Run()
	}
	// The script changed the bundle, so it needs a fresh ad-hoc signature to launch
	exec.Command("codesign", "--force", "--deep", "-s", "-", app).Run()

	if out, err := exec.Command(lsregister, "-f", app).CombinedOutput(); err != nil {
		return fmt.Errorf("lsregister: %v: %s", err, out)
	}
	return nil
}

func openBrowser(link string) error {
	return exec.Command("open", link).Run()
}

// focusTerminal brings the terminal app that launched us back to the front
func focusTerminal() {
	if id := os.Getenv("__CFBundleIdentifier"); id != "" {
		exec.Command("open", "-b", id).Run()
	}
}
