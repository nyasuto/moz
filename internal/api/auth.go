package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// DefaultJWTSecret - SECURITY WARNING: Change this in production!
	// Use environment variable JWT_SECRET for production deployments
	DefaultJWTSecret = "moz-secret-key-change-in-production" // #nosec G101
	TokenExpiration  = 24 * time.Hour
)

type AuthManager struct {
	jwtSecret []byte
	apiKeys   map[string]bool
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func NewAuthManager() *AuthManager {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = DefaultJWTSecret
	}

	return &AuthManager{
		jwtSecret: []byte(secret),
		apiKeys:   make(map[string]bool),
	}
}

func (am *AuthManager) GenerateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based key if crypto/rand fails
		return hex.EncodeToString([]byte(fmt.Sprintf("fallback-%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(bytes)
}

func (am *AuthManager) AddAPIKey(key string) {
	am.apiKeys[key] = true
}

func (am *AuthManager) GenerateJWT(username string) (string, time.Time, error) {
	expirationTime := time.Now().Add(TokenExpiration)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "moz-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(am.jwtSecret)
	return tokenString, expirationTime, err
}

func (am *AuthManager) ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return am.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check
		if c.Request.URL.Path == "/api/v1/health" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			s.errorResponse(c, http.StatusUnauthorized, "MISSING_AUTH", "Authorization header required")
			c.Abort()
			return
		}

		// Check for Bearer token
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := s.auth.ValidateJWT(tokenString)
			if err != nil {
				s.errorResponse(c, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
				c.Abort()
				return
			}
			c.Set("username", claims.Username)
			c.Next()
			return
		}

		// Check for API Key
		if strings.HasPrefix(authHeader, "ApiKey ") {
			apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
			if !s.auth.apiKeys[apiKey] {
				s.errorResponse(c, http.StatusUnauthorized, "INVALID_API_KEY", "Invalid API key")
				c.Abort()
				return
			}
			c.Set("auth_type", "api_key")
			c.Next()
			return
		}

		s.errorResponse(c, http.StatusUnauthorized, "INVALID_AUTH_FORMAT", "Authorization header must be 'Bearer <token>' or 'ApiKey <key>'")
		c.Abort()
	}
}

func (s *Server) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Simple username/password validation (in production, use proper authentication)
	if req.Username == "" || req.Password == "" {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_CREDENTIALS", "Username and password required")
		return
	}

	// Demo credentials (replace with real authentication)
	if req.Username != "admin" || req.Password != "password" {
		s.errorResponse(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password")
		return
	}

	token, expiresAt, err := s.auth.GenerateJWT(req.Username)
	if err != nil {
		s.errorResponse(c, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", err.Error())
		return
	}

	s.successResponse(c, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}, 0)
}
