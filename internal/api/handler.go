package api

import (
	"net/http"

	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-gonic/gin"
)

type ShortenRequest struct {
	URL string `json:"url" binding:"required"`
}

func ShortenHandler(svc *service.ShortenerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ShortenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		shortURL, err := svc.Shorten(req.URL)
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
