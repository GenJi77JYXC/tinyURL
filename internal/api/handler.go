package api

import (
	"net/http"
	"strconv"

	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-gonic/gin"
)

func ShortenHandler(svc *service.ShortenerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user id not found"})
			return
		}

		var req service.ShortenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		shortURL, err := svc.Shorten(req, userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"short_url": shortURL})
	}
}

func RedirectHandler(svc *service.ShortenerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		shortCode := c.Param("short")
		originalURL, err := svc.GetRedirectURL(shortCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "short url not found"})
			return
		}

		c.Redirect(http.StatusMovedPermanently, originalURL)
	}
}

func StatsHandler(svc *service.ShortenerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		shortCode := c.Param("short")
		clicks, err := svc.GetStats(shortCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"short_code": shortCode, "total_clicks": clicks})
	}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func RegisterHandler(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := auth.Register(req.Username, req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "user registered"})
	}
}

func LoginHandler(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		token, err := auth.Login(req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func MyLinksHandler(svc *service.ShortenerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user id not found"})
			return
		}

		pageStr := c.Query("page")
		limitStr := c.Query("limit")

		page, _ := strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 {
			limit = 20
		}

		links, err := svc.GetMyLinks(userID.(int64), page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"links": links})
	}
}
