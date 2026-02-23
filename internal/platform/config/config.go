package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName      string
	Environment  string
	LogLevel     string
	HTTPAddr     string
	GRPCAddr     string
	CORSOrigins  string
	MigrationsDir string
	DatabaseURL  string
	NATSURL      string
	NATSStream   string
	NATSSubject  string
	S3Endpoint   string
	S3AccessKey  string
	S3SecretKey  string
	S3Bucket     string
	S3UseSSL     bool
	S3PublicURL  string
	OTLPEndpoint string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppName:      getString("JOT_APP_NAME", "jot-backend"),
		Environment:  getString("JOT_ENV", "dev"),
		LogLevel:     getString("JOT_LOG_LEVEL", "info"),
		HTTPAddr:     getString("JOT_HTTP_ADDR", ":8080"),
		GRPCAddr:     getString("JOT_GRPC_ADDR", ":9090"),
		CORSOrigins:  getString("JOT_CORS_ORIGINS", "http://localhost:5173,http://localhost:4173,http://localhost:3000"),
		MigrationsDir: getString("JOT_MIGRATIONS_DIR", ""),
		DatabaseURL:  getString("JOT_DATABASE_URL", "postgres://jot:jot@localhost:5432/jot?sslmode=disable"),
		NATSURL:      getString("JOT_NATS_URL", "nats://localhost:4222"),
		NATSStream:   getString("JOT_NATS_STREAM", "JOT_EVENTS"),
		NATSSubject:  getString("JOT_NATS_SUBJECT", "jot.pages.events"),
		S3Endpoint:   getString("JOT_S3_ENDPOINT", "localhost:9000"),
		S3AccessKey:  getString("JOT_S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:  getString("JOT_S3_SECRET_KEY", "minioadmin"),
		S3Bucket:     getString("JOT_S3_BUCKET", "jot-media"),
		S3UseSSL:     getBool("JOT_S3_USE_SSL", false),
		S3PublicURL:  getString("JOT_S3_PUBLIC_URL", "http://localhost:9000/jot-media"),
		OTLPEndpoint: getString("JOT_OTLP_ENDPOINT", "otel-collector:4317"),
		ReadTimeout:  getDuration("JOT_READ_TIMEOUT_SEC", 10),
		WriteTimeout: getDuration("JOT_WRITE_TIMEOUT_SEC", 10),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("JOT_DATABASE_URL is required")
	}
	return cfg, nil
}

func getString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallbackSeconds int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallbackSeconds) * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(fallbackSeconds) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func getBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}
