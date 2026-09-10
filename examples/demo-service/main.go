package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "demo-service",
			"time":    time.Now().Format(time.RFC3339),
			"health":  true,
		})
	})

	r.POST("/tools/query_sales/call", func(c *gin.Context) {
		var payload map[string]any

		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid json",
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"month":       "2026-08",
			"total_sales": 128000,
			"region":      "华东",
			"tool":        "query_sales",
			"request":     payload,
		})
	})

	return r
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	if err := setupRouter().Run(":" + port); err != nil {
		panic(err)
	}
}

