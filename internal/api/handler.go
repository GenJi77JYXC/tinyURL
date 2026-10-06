package api

import (
	"errors"
	"net/http"

	"github.com/GenJi77JYXC/tinyurl/internal/model"
	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler holds the HTTP handlers.
type Handler struct {
	svc *service.Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

type shortenRequest struct {
	URL string `json:"url" binding:"required"`
}

// Shorten handles POST /shorten -> 201 with the generated short URL.
func (h *Handler) Shorten(c *gin.Context) {
	var req shortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must contain a non-empty \"url\" field"})
		return
	}

	shortURL, link, err := h.svc.Shorten(c.Request.Context(), req.URL)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidURL),
			errors.Is(err, service.ErrInvalidScheme),
			errors.Is(err, service.ErrBlockedHost),
			errors.Is(err, service.ErrUnresolvable):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create short link"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"short_code": link.ShortCode,
		"short_url":  shortURL,
	})
}

// Redirect handles GET /s/{code} with a temporary 302 (never 301: we need
// every click to pass through the service for stats and traffic shifting).
func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")

	originalURL, err := h.svc.Resolve(c.Request.Context(), code)
	if errors.Is(err, model.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "short link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "redirect lookup failed"})
		return
	}

	c.Redirect(http.StatusFound, originalURL)
}

// Healthz is the liveness/readiness probe used by Docker and compose.
func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
