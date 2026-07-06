package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file loaded")
	}

	router := gin.Default()
	router.GET("/", dashboardHandler)
	router.GET("/months/:slug", monthHandler)
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.POST("/webhook/cloudmailin", cloudMailinWebhook)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}

	if err := router.Run("0.0.0.0:" + port); err != nil {
		log.Fatal(err)
	}
}
