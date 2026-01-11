package service

import (
	"errors"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/config"
	"github.com/GenJi77JYXC/tinyurl/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo *repository.SQLiteRepo
}

func NewAuthService(repo *repository.SQLiteRepo) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) Register(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = s.repo.Db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

func (s *AuthService) Login(username, password string) (string, error) {
	var hash string
	var id int64
	err := s.repo.Db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).
		Scan(&id, &hash)
	if err != nil {
		return "", errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", errors.New("invalid password")
	}

	// 生成 JWT
	claims := jwt.MapClaims{
		"user_id":  id,
		"username": username,
		"exp":      time.Now().Add(config.JWTExpiration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JwtSecret))
}
