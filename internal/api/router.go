package api

import (
	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-gonic/gin"
)

func SetupRouter(svc *service.ShortenerService, authSvc *service.AuthService) *gin.Engine {
	r := gin.Default()

	//r.POST("/api/shorten", ShortenHandler(svc))
	r.GET("/:short", RedirectHandler(svc))
	r.GET("/api/stats/:short", StatsHandler(svc))

	r.POST("/api/register", RegisterHandler(authSvc))
	r.POST("/api/login", LoginHandler(authSvc))

	// 保护路由
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	{
		protected.POST("/shorten", ShortenHandler(svc))
		protected.GET("/my-links", MyLinksHandler(svc))
	}

	return r
}
