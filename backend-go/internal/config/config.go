package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppPort           string
	DatabaseURL       string
	RedisURL          string
	MLServiceURL      string
	OpenRouterBaseURL string
	OpenRouterAPIKey  string
	OpenRouterModel   string
	KGISBaseURL       string
	KGISTimeout       time.Duration
	KGISRetryCount    int
	KGISCacheTTL      time.Duration
	MLCacheTTL        time.Duration
	RequestTimeout    time.Duration
	MLTimeout         time.Duration
	OpenRouterTimeout time.Duration
	FrontendOrigin    string
	LogLevel          string
	DefaultSurveyType string
}

func Load() Config {
	return Config{
		AppPort:           getEnv("BACKEND_PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://landrisk:landrisk@postgres:5432/landrisk?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://redis:6379/0"),
		MLServiceURL:      getEnv("ML_SERVICE_URL", "http://ml-service:8000"),
		OpenRouterBaseURL: getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenRouterAPIKey:  getEnv("OPENROUTER_API_KEY", ""),
		OpenRouterModel:   getEnv("OPENROUTER_MODEL", "openai/gpt-oss-120b:free"),
		KGISBaseURL:       getEnv("KGIS_BASE_URL", "https://kgis.ksrsac.in:9000"),
		KGISTimeout:       time.Duration(getEnvInt("KGIS_TIMEOUT_SECONDS", 8)) * time.Second,
		KGISRetryCount:    getEnvInt("KGIS_RETRY_COUNT", 3),
		KGISCacheTTL:      time.Duration(getEnvInt("KGIS_CACHE_TTL_SECONDS", 1800)) * time.Second,
		MLCacheTTL:        time.Duration(getEnvInt("ML_CACHE_TTL_SECONDS", 1800)) * time.Second,
		RequestTimeout:    time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 25)) * time.Second,
		MLTimeout:         time.Duration(getEnvInt("ML_TIMEOUT_SECONDS", 10)) * time.Second,
		OpenRouterTimeout: time.Duration(getEnvInt("OPENROUTER_TIMEOUT_SECONDS", 8)) * time.Second,
		FrontendOrigin:    getEnv("FRONTEND_ORIGIN", "http://localhost:3000"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		DefaultSurveyType: getEnv("DEFAULT_SURVEY_TYPE", "parcel"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
