package main

import (
	"flag"
	"fmt"
	"minie4/itslearningdl/itslearning"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gosuri/uiprogress"
	"github.com/joho/godotenv"
	"github.com/muesli/termenv"
)

var max_workers = 10
var out_path = "./out/"
var logger *log.Logger

type FSResource struct {
	resource itslearning.Resource
	fsPath   string
}

func watchForChanges(orig []itslearning.Course, itsl itslearning.Itslearning) itslearning.Course {
	for {
		time.Sleep(2 * time.Second)
		logger.Info("Checking for changes")
		newCourses := itslearning.QueryCourseList(itsl)
		for i, course := range orig {
			if course.LastUpdatedUtc != newCourses[i].LastUpdatedUtc {
				logger.Info("Found change", "course", course.Title, "id", course.CourseId)
				return course
			}
		}
		logger.Debug("No changes")
	}
}

func main() {
	if handleURLMode() {
		return
	}
	var doWatch = flag.Bool("w", false, "Watch for changes")
	var doSSO = flag.Bool("sso", false, "Log in through the browser (SSO) instead of username/password")
	flag.Parse()

	bars := uiprogress.New()
	bars.Start()

	logger = log.New(bars.Bypass())
	logger.SetColorProfile(termenv.ANSI256)
	logger.SetLevel(log.DebugLevel)

	err := godotenv.Load()
	if err != nil {
		logger.Warn("Error loading .env file")
	}

	user_agent := "com.itslearning.itslearningintapp 3.7.1 (HONOR BLN-L21 / Android 9)"
	username := os.Getenv("ITSLEARNING_USERNAME")
	password := os.Getenv("ITSLEARNING_PASSWORD")
	instance := strings.TrimRight(os.Getenv("ITSLEARNING_INSTANCE"), "/")

	var itsl itslearning.Itslearning
	if *doSSO || (username == "" && password == "") {
		if instance == "" {
			logger.Fatal("ITSLEARNING_INSTANCE is missing!")
		}
		itsl = ssoLogin(instance, user_agent)
	} else {
		if username == "" || password == "" || instance == "" {
			logger.Fatal("Environment variables missing!")
		}
		itsl = itslearning.New(username, password, instance, user_agent)
	}
	courses := itslearning.QueryCourseList(itsl)
	if *doWatch {
		logger.Info("Entering watch mode. Checking for changes every :2 seconds...")
		courses = []itslearning.Course{watchForChanges(courses, itsl)}
	}

	bar_course := bars.AddBar(len(courses)).AppendFunc(appendProgress)
	bar_course.PrependFunc(func(b *uiprogress.Bar) string {
		return "Total progress"
	})
	for _, course := range courses {
		logger.Info("Downloading course", "course", course.Title, "id", course.CourseId)

		var resouceList []FSResource
		bars.Bars = []*uiprogress.Bar{bar_course}
		bar := bars.AddBar(1)
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return "Fetching files"
		})
		res := itslearning.QueryCourseResources(itsl, course.CourseId)
		for _, resource := range res {
			var coursePath = sanitizePath(course.Title) + "/" + sanitizePath(resource.Title)

			if resource.ElementType == "Folder" {
				fetchResourcesRecursive(itsl, course, resource.ElementID, coursePath, &resouceList)
				logger.Info("Found folder", "element", resource.Title, "id", resource.ElementID)
			}

			if resource.ElementType == "LearningToolElement" {
				var fsPath = out_path + coursePath
				var fsResource = FSResource{resource: resource, fsPath: fsPath}
				resouceList = append(resouceList, fsResource)
				logger.Info("Found element", "element", resource.Title, "id", resource.ElementID)
			}
		}
		bar.Incr()

		if len(resouceList) < 1 {
			continue
		}

		var wg sync.WaitGroup
		current_tasks := 0
		bars.Bars = []*uiprogress.Bar{bar_course}
		bar = bars.AddBar(len(resouceList)).AppendFunc(appendProgress)
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return "Downloading files"
		})
		for _, fsResource := range resouceList {
			var resource = fsResource.resource
			var fsPath = fsResource.fsPath
			if _, err := os.Stat(fsPath); err == nil {
				logger.Info("Skipping element", "course", course.Title, "element", resource.Title, "id", resource.ElementID)
				continue
			}
			logger.Info("Downloading element", "course", course.Title, "element", resource.Title, "id", resource.ElementID)
			wg.Add(1)
			current_tasks++
			go func(resource itslearning.Resource) {
				defer wg.Done()
				itslearning.DownloadElement(itsl, resource.ElementID, fsPath)
				bar.Incr()
				current_tasks--
			}(resource)

			if current_tasks >= max_workers {
				logger.Debug("Waiting for running downloads to finish")
				wg.Wait()
			}
		}
		wg.Wait()
		bar.Set(100)
		bar = nil

		bar_course.Incr()
	}
	bars.Stop()
	logger.Info("Done")
	os.Exit(0)
}

func fetchResourcesRecursive(itsl itslearning.Itslearning, course itslearning.Course, folder_resource int, coursePath string, resList *[]FSResource) {
	res := itslearning.QueryFolderResources(itsl, course.CourseId, folder_resource)
	for _, resource := range res {
		var newCoursePath = coursePath + "/" + sanitizePath(resource.Title)

		if resource.ElementType == "Folder" {
			fetchResourcesRecursive(itsl, course, resource.ElementID, newCoursePath, resList)
			logger.Info("Found subfolder", "element", resource.Title, "id", resource.ElementID)
		}

		if resource.ElementType == "LearningToolElement" {
			var fsPath = out_path + newCoursePath
			var fsResource = FSResource{resource: resource, fsPath: fsPath}
			*resList = append(*resList, fsResource)
			logger.Info("Found element", "element", resource.Title, "id", resource.ElementID)
		}
	}
}

func sanitizePath(path string) string {
	// Remove leading slash of filepath
	var regex = regexp.MustCompile(`^\s*?/\s*`)
	var sanitizedWhitespace = regex.ReplaceAllString(path, "")

	// Replace invalid filename characters with underscore
	var invalidCharsRegex = regexp.MustCompile(`[<>:"/\|?*]`)
	return invalidCharsRegex.ReplaceAllString(sanitizedWhitespace, "_")
}

func appendProgress(b *uiprogress.Bar) string {
	return fmt.Sprintf("%d/%d (%.2f%%)", b.Current(), b.Total, (float32(b.Current())/float32(b.Total))*100.0)
}
