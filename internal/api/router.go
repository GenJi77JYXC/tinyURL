package api

import (
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(svc *service.ShortenerService, authSvc *service.AuthService) *gin.Engine {
	r := gin.Default()

	// 添加 CORS 中间件（允许本地开发跨域）
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:5174", "https://mahiro.cloud", "https://www.mahiro.cloud"}, // 允许这些源
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	//r.POST("/api/shorten", ShortenHandler(svc))
	r.GET("/:short", RedirectHandler(svc))
	r.GET("/api/stats/:short", StatsHandler(svc))

	r.POST("/api/register", RegisterHandler(authSvc))
	r.POST("/api/login", LoginHandler(authSvc))

	// 保护路由
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	protected.Use(RateLimitMiddleware())
	{
		protected.POST("/shorten", ShortenHandler(svc))
		protected.GET("/my-links", MyLinksHandler(svc))
	}

	return r
}
