package config

import (
	"fmt"
	"os"
	"strconv"
)

type Service struct {
	Name string
	URL  string
}

type Config struct {
	Services           []Service
	PollInterval       int // seconds
	HTTPTimeout        int // seconds
	DBPath             string
	DataRetentionDays  int
	Port               string
	HeimdallUser       string
	HeimdallPassword   string
}

func Load() (*Config, error) {
	cfg := &Config{
		PollInterval:      60,
		HTTPTimeout:       10,
		DBPath:            "./heimdall.db",
		DataRetentionDays: 90,
		Port:              "8080",
	}

	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid POLL_INTERVAL: %q", v)
		}
		cfg.PollInterval = n
	}

	if v := os.Getenv("HTTP_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HTTP_TIMEOUT: %q", v)
		}
		cfg.HTTPTimeout = n
	}

	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}

	if v := os.Getenv("DATA_RETENTION_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid DATA_RETENTION_DAYS: %q", v)
		}
		cfg.DataRetentionDays = n
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}

	cfg.HeimdallUser = os.Getenv("HEIMDALL_USER")
	cfg.HeimdallPassword = os.Getenv("HEIMDALL_PASSWORD")

	for i := 1; ; i++ {
		name := os.Getenv(fmt.Sprintf("SERVICE_%d_NAME", i))
		url := os.Getenv(fmt.Sprintf("SERVICE_%d_URL", i))
		if name == "" && url == "" {
			break
		}
		if name == "" || url == "" {
			return nil, fmt.Errorf("SERVICE_%d: both NAME and URL must be set", i)
		}
		cfg.Services = append(cfg.Services, Service{Name: name, URL: url})
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("no services configured: set SERVICE_1_NAME and SERVICE_1_URL")
	}

	return cfg, nil
}

// BasicAuthEnabled returns true only when both user and password are configured.
func (c *Config) BasicAuthEnabled() bool {
	return c.HeimdallUser != "" && c.HeimdallPassword != ""
}
