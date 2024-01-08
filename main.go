package main

import (
	"fmt"
	"minie4/itslearningdl/itslearning"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gosuri/uiprogress"
	"github.com/joho/godotenv"
	"github.com/muesli/termenv"
)

var max_workers = 10
var logger *log.Logger

func main() {
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
	instance := os.Getenv("ITSLEARNING_INSTANCE")

	if username == "" || password == "" || instance == "" {
		logger.Fatal("Environment variables missing!")
	}

	itsl := itslearning.New(username, password, instance, user_agent)
	courses := itslearning.QueryCourseList(itsl)

	bar_course := bars.AddBar(len(courses)).AppendFunc(appendProgress)
	bar_course.PrependFunc(func(b *uiprogress.Bar) string {
		return "Total progress"
	})
	for _, course := range courses {
		logger.Info("Downloading course", "course", course.Title, "id", course.CourseId)

		var resouceList []itslearning.Resource
		bars.Bars = []*uiprogress.Bar{bar_course}
		bar := bars.AddBar(1)
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return "Fetching files"
		})
		res := itslearning.QueryCourseResources(itsl, course.CourseId)
		for _, resource := range res {
			if resource.ElementType == "Folder" {
				fetchResourcesRecursive(itsl, course, resource.ElementID, &resouceList)
				logger.Info("Found folder", "element", resource.Title, "id", resource.ElementID)
			}

			if resource.ElementType == "LearningToolElement" {
				resouceList = append(resouceList, resource)
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
		for _, resource := range resouceList {
			var out_path = "./out/" + course.Title + "/" + resource.Title
			if _, err := os.Stat(out_path); err == nil {
				logger.Info("Skipping element", "course", course.Title, "element", resource.Title, "id", resource.ElementID)
				continue
			}
			logger.Info("Downloading element", "course", course.Title, "element", resource.Title, "id", resource.ElementID)
			wg.Add(1)
			current_tasks++
			go func(resource itslearning.Resource) {
				defer wg.Done()
				itslearning.DownloadElement(itsl, resource.ElementID, out_path)
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
}

func fetchResourcesRecursive(itsl itslearning.Itslearning, course itslearning.Course, folder_resource int, resList *[]itslearning.Resource) {
	res := itslearning.QueryFolderResources(itsl, course.CourseId, folder_resource)
	for _, resource := range res {
		if resource.ElementType == "Folder" {
			fetchResourcesRecursive(itsl, course, resource.ElementID, resList)
			logger.Info("Found subfolder", "element", resource.Title, "id", resource.ElementID)
		}

		if resource.ElementType == "LearningToolElement" {
			*resList = append(*resList, resource)
			logger.Info("Found element", "element", resource.Title, "id", resource.ElementID)
		}
	}
}

func appendProgress(b *uiprogress.Bar) string {
	return fmt.Sprintf("%d/%d (%.2f%%)", b.Current(), b.Total, (float32(b.Current())/float32(b.Total))*100.0)
}
