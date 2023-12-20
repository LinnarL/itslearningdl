package itslearning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/charmbracelet/log"
)

type Itslearning struct {
	instance     string
	user_agent   string
	access_token string
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

func RequestAuthtoken(itsl Itslearning, username string, password string) string {
	path := itsl.instance + "/restapi/oauth2/token"
	payload := url.Values{
		"client_id":  {"10ae9d30-1853-48ff-81cb-47b58a325685"},
		"grant_type": {"password"},
		"username":   {username},
		"password":   {password},
	}

	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(payload.Encode()))
	data := new(map[string]interface{})
	err = doHTTPReq(*req, err, itsl, "Login", data)
	return (*data)["access_token"].(string)
}

func QueryCourseList(itsl Itslearning) []Course {
	path := itsl.instance + "/restapi/personal/courses/v2"
	payload := url.Values{
		"access_token": {itsl.access_token},
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
		"access_token": {itsl.access_token},
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
		"access_token": {itsl.access_token},
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
		"access_token": {itsl.access_token},
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

	dir, _ := filepath.Split(outfile)
	os.MkdirAll(dir, os.ModePerm)
	out, err := os.Create(outfile)
	if err != nil {
		log.Error("Failed writing file!", "error", err)
	}
	defer out.Close()
	defer res.Body.Close()
	io.Copy(out, res.Body)
}

func New(username string, password string, instance string, useragent string) Itslearning {
	log.Debug("Attempting to log in")
	e := Itslearning{instance, useragent, ""}
	e.access_token = RequestAuthtoken(e, username, password)
	return e
}
