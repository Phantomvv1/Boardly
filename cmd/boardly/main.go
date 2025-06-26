package main

import (
	"net/http"
	"time"

	. "github.com/Phantomvv1/Boardly/internal/chat"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	go RunningHub.Run()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Any("/", func(c *gin.Context) { c.JSON(http.StatusOK, nil) })
	r.GET("/ws/chat", Chat)
	r.Static("/chat", "../../frontend")

	r.Run(":42069")
}
