package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShowHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func ShowImageOptimizerPage(c *gin.Context) {
	c.HTML(http.StatusOK, "image_toolbox.html", nil)
}
