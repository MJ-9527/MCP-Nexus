package handler

import (
	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) {
	respondSuccess(c, gin.H{
		"status": "ok",
	})
}
