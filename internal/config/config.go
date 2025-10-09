package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	Offline            bool
	OPAUrl             string
	SQLitePath         string
	Port               string
	LogFormat          string // "text" or "json"
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	DevMode            bool
	CORSAllowedOrigins string
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	offline := true
	if val := os.Getenv("OFFLINE"); val != "" {
		offline, _ = strconv.ParseBool(val)
	}

	opaURL := os.Getenv("OPA_URL")
	if opaURL == "" {
		opaURL = "http://localhost:8181/v1/data/accessgraph"
	}

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "./graph.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text"
	}

	readTimeout := 15 * time.Second
	if val := os.Getenv("READ_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			readTimeout = d
		}
	}

	writeTimeout := 15 * time.Second
	if val := os.Getenv("WRITE_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			writeTimeout = d
		}
	}

	idleTimeout := 60 * time.Second
	if val := os.Getenv("IDLE_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			idleTimeout = d
		}
	}

	devMode := false
	if val := os.Getenv("DEV"); val != "" {
		devMode, _ = strconv.ParseBool(val)
	}

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")

	return &Config{
		Offline:            offline,
		OPAUrl:             opaURL,
		SQLitePath:         sqlitePath,
		Port:               port,
		LogFormat:          logFormat,
		ReadTimeout:        readTimeout,
		WriteTimeout:       writeTimeout,
		IdleTimeout:        idleTimeout,
		DevMode:            devMode,
		CORSAllowedOrigins: corsOrigins,
	}
}
