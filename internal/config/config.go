package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Offline    bool
	OPAUrl     string
	SQLitePath string
	Port       string
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

	return &Config{
		Offline:    offline,
		OPAUrl:     opaURL,
		SQLitePath: sqlitePath,
		Port:       port,
	}
}

