package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"minie4/itslearningdl/itslearning"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// The refresh token is stored here so the browser login is only needed once
var token_path = ".itslearning-token.json"

type savedToken struct {
	Instance     string `json:"instance"`
	RefreshToken string `json:"refresh_token"`
}

func ssoLogin(instance string, user_agent string) itslearning.Itslearning {
	if saved, err := loadToken(); err == nil && saved.Instance == instance && saved.RefreshToken != "" {
		logger.Debug("Refreshing saved SSO login")
		token, err := itslearning.RefreshToken(instance, user_agent, saved.RefreshToken)
		if err == nil {
			saveToken(instance, token)
			return newSSOClient(instance, user_agent, token)
		}
		logger.Warn("Saved login expired, logging in again", "error", err)
	}

	stateBytes := make([]byte, 8)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	authURL := itslearning.AuthorizeURL(instance, state)

	redirects := make(chan string, 1)
	auto := startCallbackServer(redirects)

	if openBrowser(authURL) != nil {
		fmt.Fprintf(os.Stderr, "\nOpen this link to log in:\n\n   %s\n", authURL)
	}
	if auto {
		fmt.Fprint(os.Stderr, `
SSO login: sign in with your school account in the browser window that just opened.
When the browser asks to open "Itslearning Login", click Open (tick "Always allow"
to skip that next time). This terminal continues automatically.

If nothing happens, paste the itsl-itslearning://login?code=... URL here: `)
	} else {
		fmt.Fprint(os.Stderr, `
SSO login: sign in with your school account in the browser. The page will then fail to
open "itsl-itslearning://login?code=..." (visible in the DevTools console, Cmd+Option+J,
with "Preserve log" ticked). Paste that whole address here: `)
	}

	// Whichever arrives first: the browser redirect or a pasted URL
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				redirects <- line
				return
			}
		}
	}()

	var code string
	for code == "" {
		redirect := <-redirects
		c, err := extractCode(redirect, state)
		if err != nil {
			logger.Error("Invalid redirect URL, try again", "error", err)
			continue
		}
		code = c
	}
	fmt.Fprintln(os.Stderr)
	focusTerminal()

	token, err := itslearning.ExchangeCode(instance, user_agent, code)
	if err != nil {
		logger.Fatal("SSO login failed", "error", err)
	}
	saveToken(instance, token)
	logger.Info("SSO login successful")
	return newSSOClient(instance, user_agent, token)
}

// The URL handler app posts the itsl-itslearning:// redirect to this local address
const callbackAddr = "127.0.0.1:47219"

// startCallbackServer listens for redirects forwarded by the URL handler app.
// It returns false if the automatic redirect could not be set up.
func startCallbackServer(redirects chan<- string) bool {
	ln, err := net.Listen("tcp", callbackAddr)
	if err != nil {
		logger.Warn("Could not start login callback server", "error", err)
		return false
	}
	if err := installURLHandler("http://" + callbackAddr + "/callback"); err != nil {
		logger.Warn("Could not install login URL handler", "error", err)
		ln.Close()
		return false
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if u := r.FormValue("url"); u != "" {
			select {
			case redirects <- u:
			default:
			}
		}
		fmt.Fprintln(w, "ok")
	})
	go http.Serve(ln, mux)
	return true
}

func newSSOClient(instance string, user_agent string, token itslearning.Token) itslearning.Itslearning {
	logger.Debug("Access token lifetime", "seconds", token.ExpiresIn)
	return itslearning.NewWithToken(instance, user_agent, token, func(t itslearning.Token) {
		saveToken(instance, t)
	})
}

func extractCode(input string, state string) (string, error) {
	// Chrome's console message wraps the URL in quotes
	input = strings.Trim(input, "'\" ")
	if !strings.Contains(input, "code=") {
		return "", fmt.Errorf("no code= found in %q", input)
	}
	u, err := url.Parse(input)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if s := q.Get("state"); s != "" && s != state {
		return "", fmt.Errorf("state mismatch, please use the link from this run")
	}
	return q.Get("code"), nil
}

func loadToken() (savedToken, error) {
	var saved savedToken
	data, err := os.ReadFile(token_path)
	if err != nil {
		return saved, err
	}
	err = json.Unmarshal(data, &saved)
	return saved, err
}

func saveToken(instance string, token itslearning.Token) {
	if token.RefreshToken == "" {
		return
	}
	data, _ := json.Marshal(savedToken{instance, token.RefreshToken})
	if err := os.WriteFile(token_path, data, 0600); err != nil {
		logger.Warn("Could not save login token", "error", err)
	}
}

// handleURLMode is run when the browser launches us for an itsl-itslearning:// link
// (Windows): it forwards the link to the waiting login process and exits.
func handleURLMode() bool {
	if len(os.Args) != 3 || os.Args[1] != "--handle-url" {
		return false
	}
	resp, err := http.PostForm("http://"+callbackAddr+"/callback", url.Values{"url": {os.Args[2]}})
	if err == nil {
		resp.Body.Close()
	}
	return true
}
