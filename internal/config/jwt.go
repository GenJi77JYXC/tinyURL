package config

import (
	"os"
	"time"
)

var (
	JwtSecret     string // 生产环境用环境变量！
	JWTExpiration = 24 * time.Hour
)

func init() {
	JwtSecret = os.Getenv("JWT_SECRET")
}
