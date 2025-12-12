package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port string
	Env  string

	// Database
	DatabaseURL string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// JWT
	JWTPrivateKey          *rsa.PrivateKey
	JWTPublicKey           *rsa.PublicKey
	JWTAccessTokenExpiry   time.Duration
	JWTRefreshTokenExpiry  time.Duration

	// CORS
	AllowedOrigins []string

	// Admin
	AdminEmails []string
}

func Load() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		Env:                   getEnv("ENV", "development"),
		DatabaseURL:           getEnv("DATABASE_URL", ""),
		GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:     getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		AllowedOrigins:        parseCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		AdminEmails:           parseCSV(getEnv("ADMIN_EMAILS", "")),
	}

	// Parse JWT token expiry
	accessExpiry := getEnv("JWT_ACCESS_TOKEN_EXPIRY", "15m")
	var err error
	cfg.JWTAccessTokenExpiry, err = time.ParseDuration(accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TOKEN_EXPIRY: %w", err)
	}

	refreshExpiry := getEnv("JWT_REFRESH_TOKEN_EXPIRY", "168h")
	cfg.JWTRefreshTokenExpiry, err = time.ParseDuration(refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TOKEN_EXPIRY: %w", err)
	}

	// Load or generate RSA keys
	privateKeyPath := getEnv("JWT_PRIVATE_KEY_PATH", "./keys/private_key.pem")
	publicKeyPath := getEnv("JWT_PUBLIC_KEY_PATH", "./keys/public_key.pem")

	cfg.JWTPrivateKey, cfg.JWTPublicKey, err = loadOrGenerateKeys(privateKeyPath, publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load RSA keys: %w", err)
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if cfg.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func loadOrGenerateKeys(privateKeyPath, publicKeyPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	// Try to load existing keys
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err == nil {
		publicKey, err := loadPublicKey(publicKeyPath)
		if err == nil {
			return privateKey, publicKey, nil
		}
	}

	// If keys don't exist or failed to load, generate new ones
	return generateAndSaveKeys(privateKeyPath, publicKeyPath)
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS8 format first (BEGIN PRIVATE KEY)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("key is not RSA private key")
	}

	// Try PKCS1 format (BEGIN RSA PRIVATE KEY)
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

func generateAndSaveKeys(privateKeyPath, publicKeyPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	// This is a placeholder - keys should be generated using external tools
	// For now, return an error prompting the user to generate keys
	return nil, nil, fmt.Errorf("RSA keys not found at %s and %s. Please generate them using:\n  openssl genrsa -out %s 4096\n  openssl rsa -in %s -pubout -out %s",
		privateKeyPath, publicKeyPath, privateKeyPath, privateKeyPath, publicKeyPath)
}
