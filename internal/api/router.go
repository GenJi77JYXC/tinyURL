package api

import (
	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter(svc *service.ShortenerService) *gin.Engine {
	r := gin.Default()

	r.POST("/api/shorten", ShortenHandler(svc))
	r.GET("/:short", RedirectHandler(svc))
	r.GET("/api/stats/:short", StatsHandler(svc))

	return r
}
