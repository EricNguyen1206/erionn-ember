package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultPort        = "9090"
	DefaultMaxConns    = 10000
	DefaultIdleTimeout = 5 * time.Minute
)

type Config struct {
	Port        string
	MaxConns    int
	IdleTimeout time.Duration
}

func Default() *Config {
	return &Config{
		Port:        DefaultPort,
		MaxConns:    DefaultMaxConns,
		IdleTimeout: DefaultIdleTimeout,
	}
}

func (c *Config) OverrideFromEnv() {
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("MAX_CONNECTIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxConns = n
		}
	}
	if v := os.Getenv("IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.IdleTimeout = d
		}
	}
}
