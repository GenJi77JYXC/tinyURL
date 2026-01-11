package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(config.JwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		userID := int64(claims["user_id"].(float64))
		c.Set("user_id", userID)
		c.Next()
	}
}

func RateLimitMiddleware() gin.HandlerFunc {
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  60, // 每分钟 60 次
	}
	store := memory.NewStore()
	limiter := limiter.New(store, rate)

	middleware := mgin.NewMiddleware(limiter, mgin.WithKeyGetter(func(c *gin.Context) string {
		// IP + 用户ID（如果登录）
		ip := c.ClientIP()
		userID, _ := c.Get("user_id")
		if userID != nil {
			return fmt.Sprintf("%s:%d", ip, userID.(int64))
		}
		return ip
	}))

	return middleware
}
