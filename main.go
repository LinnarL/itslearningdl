package main

import (
	"minie4/itslearningdl/itslearning"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

func main() {

	// log.SetLevel(log.DebugLevel)

	err := godotenv.Load()
	if err != nil {
		log.Warn("Error loading .env file")
	}

	user_agent := "com.itslearning.itslearningintapp 3.7.1 (HONOR BLN-L21 / Android 9)"
	username := os.Getenv("ITSLEARNING_USERNAME")
	password := os.Getenv("ITSLEARNING_PASSWORD")
	instance := os.Getenv("ITSLEARNING_INSTANCE")

	if username == "" || password == "" || instance == "" {
		log.Fatal("Environment variables missing!")
	}

	itsl := itslearning.New(username, password, instance, user_agent)

	courses := itslearning.QueryCourseList(itsl)
	for _, course := range courses {
		log.Info("Downloading course", "course", course.Title, "id", course.CourseId)
		res := itslearning.QueryCourseResources(itsl, course.CourseId)

		for _, resource := range res {
			if resource.ElementType != "LearningToolElement" {
				continue
			}
			log.Info("Downloading element", "element", resource.Title, "id", resource.ElementID)
			itslearning.DownloadElement(itsl, resource.ElementID, "./out/"+course.Title+"/"+resource.Title)
		}
	}

}
