package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/api"
	"github.com/GenJi77JYXC/tinyurl/internal/repository"
	"github.com/GenJi77JYXC/tinyurl/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "./tinyurl.db"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	repo, err := repository.NewSQLiteRepo(dbPath)
	if err != nil {
		log.Fatalf("Failed to init SQLite: %v", err)
	}
	defer repo.Close()

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPass := ""
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB")) // 默认 0
	redisTTLStr := os.Getenv("REDIS_TTL")
	redisTTL, _ := time.ParseDuration(redisTTLStr) // 如 "720h"

	redisRepo := repository.NewRedisRepo(redisAddr, redisPass, redisDB, redisTTL)

	svc := service.NewShortenerService(repo, redisRepo, baseURL, 8) // 短码长度

	r := api.SetupRouter(svc)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("TinyURL server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
