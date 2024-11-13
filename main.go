package main

import (
	"github.com/gin-gonic/gin"
	"github.com/herrkruger/picoptim/controllers"
)

func main() {
	//gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./static")
	router.Static("/downloads", "./downloads")

	// Define routes
	router.GET("/", controllers.ShowHomePage)
	router.GET("/image-toolbox", controllers.ShowImageOptimizerPage)
	router.POST("/download-zip", controllers.DownloadZip)
	router.POST("/optimize", controllers.OptimizeImage)

	router.Run(":8080")

}
