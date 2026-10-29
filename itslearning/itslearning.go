package itslearning

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/charmbracelet/log"
)

type Itslearning struct {
	instance   string
	user_agent string
	auth       *authState
}

// authState is shared between all copies of an Itslearning value so that
// a refreshed token is picked up by every running download
type authState struct {
	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiry       time.Time
	onRefresh    func(Token)
}

// token returns a valid access token, refreshing it shortly before it expires
func (itsl Itslearning) token() string {
	a := itsl.auth
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.refreshToken != "" && time.Now().After(a.expiry) {
		log.Debug("Refreshing access token")
		token, err := RefreshToken(itsl.instance, itsl.user_agent, a.refreshToken)
		if err != nil {
			log.Error("Token refresh failed", "error", err)
		} else {
			a.set(token)
			if a.onRefresh != nil {
				a.onRefresh(token)
			}
		}
	}
	return a.accessToken
}

func (a *authState) set(token Token) {
	a.accessToken = token.AccessToken
	if token.RefreshToken != "" {
		a.refreshToken = token.RefreshToken
	}
	lifetime := time.Duration(token.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = 10 * time.Minute
	}
	// Refresh a bit early so requests never use an expired token
	a.expiry = time.Now().Add(lifetime * 3 / 4)
}

type Course struct {
	CourseCode            string    `json:"CourseCode"`
	CourseColor           string    `json:"CourseColor"`
	CourseFillColor       string    `json:"CourseFillColor"`
	CourseId              int       `json:"CourseId"`
	FriendlyName          *string   `json:"FriendlyName"`
	HasAdminPermissions   bool      `json:"HasAdminPermissions"`
	HasStudentPermissions bool      `json:"HasStudentPermissions"`
	LastUpdatedUtc        time.Time `json:"LastUpdatedUtc"`
	NewBulletinsCount     int       `json:"NewBulletinsCount"`
	NewNotificationsCount int       `json:"NewNotificationsCount"`
	Title                 string    `json:"Title"`
	Url                   string    `json:"Url"`
}

type Resource struct {
	Title                    string `json:"Title"`
	ElementID                int    `json:"ElementId"`
	ElementType              string `json:"ElementType"`
	CourseID                 int    `json:"CourseId"`
	URL                      string `json:"Url"`
	ContentURL               string `json:"ContentUrl"`
	IconURL                  string `json:"IconUrl"`
	Active                   bool   `json:"Active"`
	LearningToolID           int    `json:"LearningToolId"`
	AddElementURL            any    `json:"AddElementUrl"`
	Homework                 bool   `json:"Homework"`
	Path                     string `json:"Path"`
	LearningObjectID         int    `json:"LearningObjectId"`
	LearningObjectInstanceID int    `json:"LearningObjectInstanceId"`
}

func doHTTPReq(req http.Request, err error, itsl Itslearning, method string, target interface{}) error {
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", itsl.user_agent)
	if err != nil {
		log.Warn("Itslearning request failed!", "method", method, "error", err)
		return err
	}

	res, err := http.DefaultClient.Do(&req)
	if err != nil {
		log.Warn("Itslearning request failed!", "method", method, "error", err)
		return err
	}

	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(target)
}

const ClientID = "10ae9d30-1853-48ff-81cb-47b58a325685"

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func requestToken(itsl Itslearning, method string, payload url.Values) (Token, error) {
	path := itsl.instance + "/restapi/oauth2/token"
	payload.Set("client_id", ClientID)

	var token Token
	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(payload.Encode()))
	if err != nil {
		return token, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", itsl.user_agent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return token, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return token, fmt.Errorf("%s failed (HTTP %d): %s", method, res.StatusCode, strings.TrimSpace(string(body)))
	}
	return token, nil
}

func RequestAuthtoken(itsl Itslearning, username string, password string) Token {
	token, err := requestToken(itsl, "Login", url.Values{
		"grant_type": {"password"},
		"username":   {username},
		"password":   {password},
	})
	if err != nil {
		log.Fatal("Login failed", "error", err)
	}
	return token
}

// AuthorizeURL returns the browser login page (including SSO options) used by the mobile app
func AuthorizeURL(instance string, state string) string {
	payload := url.Values{
		"client_id":     {ClientID},
		"redirect_uri":  {"itsl-itslearning://login"},
		"response_type": {"code"},
		"scope":         {"SCOPE"},
		"state":         {state},
	}
	return instance + "/oauth2/authorize.aspx?" + payload.Encode()
}

func ExchangeCode(instance string, useragent string, code string) (Token, error) {
	return requestToken(Itslearning{instance, useragent, &authState{}}, "Code exchange", url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {"itsl-itslearning://login"},
	})
}

func RefreshToken(instance string, useragent string, refreshToken string) (Token, error) {
	return requestToken(Itslearning{instance, useragent, &authState{}}, "Token refresh", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func QueryCourseList(itsl Itslearning) []Course {
	path := itsl.instance + "/restapi/personal/courses/v2"
	payload := url.Values{
		"access_token": {itsl.token()},
		"pageIndex":    {"0"},
		"pageSize":     {"9999"},
		"filter":       {"1"},
	}

	req, err := http.NewRequest(http.MethodGet, path, strings.NewReader(payload.Encode()))
	data := new(struct{ EntityArray []Course })
	doHTTPReq(*req, err, itsl, "QueryCourseList", data)
	return data.EntityArray
}

func QueryCourseResources(itsl Itslearning, courseID int) []Resource {
	path := fmt.Sprintf("%s/restapi/personal/courses/%s/resources/v1", itsl.instance, strconv.Itoa(courseID))
	payload := url.Values{
		"access_token": {itsl.token()},
		"pageIndex":    {"0"},
		"pageSize":     {"9999"},
	}

	req, err := http.NewRequest(http.MethodGet, path, strings.NewReader(payload.Encode()))
	data := new(struct {
		Resources struct {
			EntityArray []Resource
		}
	})
	doHTTPReq(*req, err, itsl, "QueryCourseResources", data)
	return data.Resources.EntityArray
}

func QueryFolderResources(itsl Itslearning, courseID int, folderElementID int) []Resource {
	path := fmt.Sprintf("%s/restapi/personal/courses/%s/folders/%s/resources/v1", itsl.instance, strconv.Itoa(courseID), strconv.Itoa(folderElementID))
	payload := url.Values{
		"access_token": {itsl.token()},
		"pageIndex":    {"0"},
		"pageSize":     {"9999"},
	}

	req, err := http.NewRequest(http.MethodGet, path, strings.NewReader(payload.Encode()))
	data := new(struct {
		Resources struct {
			EntityArray []Resource
		}
	})
	doHTTPReq(*req, err, itsl, "QueryFolderResources", data)
	return data.Resources.EntityArray
}

func DownloadElement(itsl Itslearning, elementID int, outfile string) {
	// Request the SSO redirect URL
	path := fmt.Sprintf("%s/restapi/personal/sso/url/v1", itsl.instance)
	payload := url.Values{
		"access_token": {itsl.token()},
		"url":          {fmt.Sprintf("%s/LearningToolElement/ViewLearningToolElement.aspx?LearningToolElementId=%d", itsl.instance, elementID)},
	}

	req, err := http.NewRequest(http.MethodGet, path, nil)
	req.URL.RawQuery = payload.Encode()
	data := new(struct{ Url string })
	doHTTPReq(*req, err, itsl, "QueryFolderResources", data)
	if err != nil {
		log.Error("Error fetching SSO URL:", err)
		return
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}
	log.Debug("SSO URL", "data", data)

	// Make a request to the SSO redirect
	req, err = http.NewRequest(http.MethodGet, data.Url, nil)
	req.Header.Set("User-Agent", itsl.user_agent)
	res, err := client.Do(req)
	if err != nil {
		log.Warn("File download failed!", "error", err)
		return
	}
	html, err := io.ReadAll(res.Body)

	// Get the iframe URL
	doc := soup.HTMLParse(string(html))
	iframe := doc.Find("iframe")
	if iframe.Error != nil {
		log.Error(iframe.Error)
		return
	}

	iframeSrc := iframe.Attrs()["src"]

	log.Debug("File iframe Src", "src", iframeSrc)

	req, err = http.NewRequest(http.MethodGet, iframeSrc, nil)
	req.Header.Set("User-Agent", itsl.user_agent)
	res, err = client.Do(req)
	if err != nil {
		log.Warn("File download failed!", "error", err)
		return
	}
	html, err = io.ReadAll(res.Body)

	redirectedUrl := res.Request.URL.String()
	u, _ := url.Parse(redirectedUrl)
	q, _ := url.ParseQuery(u.RawQuery)

	payload = url.Values{
		"LearningObjectId":         {q.Get("LearningObjectId")},
		"LearningObjectInstanceId": {q.Get("LearningObjectInstanceId")},
	}
	resourceUrl := "https://resource.itslearning.com/Proxy/DownloadRedirect.ashx"
	req, err = http.NewRequest(http.MethodGet, resourceUrl, nil)
	req.URL.RawQuery = payload.Encode()
	req.Header.Set("User-Agent", itsl.user_agent)
	res, err = client.Do(req)
	if err != nil {
		log.Warn("File download failed!", "error", err)
		return
	}

	defer res.Body.Close()
	if res.StatusCode >= 400 {
		log.Warn("File download failed!", "status", res.Status, "file", outfile)
		return
	}

	// Titles on itslearning usually have no file extension, so work it out from the response
	head := make([]byte, 512)
	n, _ := io.ReadFull(res.Body, head)
	head = head[:n]
	outfile = withExtension(outfile, res, head)

	dir, _ := filepath.Split(outfile)
	os.MkdirAll(dir, os.ModePerm)
	out, err := os.Create(outfile)
	if err != nil {
		log.Error("Failed writing file!", "error", err)
		return
	}
	defer out.Close()
	out.Write(head)
	io.Copy(out, res.Body)
}

// withExtension appends a file extension to outfile based on the server's filename,
// Content-Type or the file contents, unless the name already ends with it
func withExtension(outfile string, res *http.Response, head []byte) string {
	ext := ""
	if _, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition")); err == nil {
		ext = filepath.Ext(params["filename"])
	}
	if ext == "" {
		ctype := res.Header.Get("Content-Type")
		if ctype == "" || strings.HasPrefix(ctype, "application/octet-stream") {
			ctype = http.DetectContentType(head)
		}
		if mediaType, _, err := mime.ParseMediaType(ctype); err == nil {
			// Prefer the common extension where the system lists several (e.g. .jpe, .jpeg, .jpg)
			if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
				ext = exts[0]
				for _, e := range exts {
					if e == ".pdf" || e == ".jpg" || e == ".txt" || e == ".html" {
						ext = e
					}
				}
			}
		}
	}
	if ext == "" || strings.HasSuffix(strings.ToLower(outfile), strings.ToLower(ext)) {
		return outfile
	}
	return outfile + ext
}

func New(username string, password string, instance string, useragent string) Itslearning {
	log.Debug("Attempting to log in")
	e := Itslearning{instance, useragent, &authState{}}
	e.auth.set(RequestAuthtoken(e, username, password))
	return e
}

// NewWithToken creates a client that keeps itself logged in; onRefresh is called with each new token
func NewWithToken(instance string, useragent string, token Token, onRefresh func(Token)) Itslearning {
	e := Itslearning{instance, useragent, &authState{onRefresh: onRefresh}}
	e.auth.set(token)
	return e
}
