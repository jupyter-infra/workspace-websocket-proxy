/*
Copyright (c) 2026 Jupyter Infrastructure
Distributed under the terms of the MIT license
*/

package proxy

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the WebSocket proxy.
type Config struct {
	// ListenAddr is the address the HTTP server listens on.
	ListenAddr string

	// TargetHost is the TCP host to proxy to.
	TargetHost string

	// TargetPort is the TCP port to proxy to.
	TargetPort string

	// MaxSessionDuration is the maximum lifetime of a single connection.
	// After this duration, the connection is closed regardless of activity.
	MaxSessionDuration time.Duration

	// PingInterval is how often to send WebSocket ping frames.
	PingInterval time.Duration

	// PingTimeout is how long to wait for a pong response before closing.
	PingTimeout time.Duration

	// MaxConnections is the maximum number of concurrent WebSocket connections.
	// New connections are rejected with 503 when at capacity.
	MaxConnections int

	// RevalidationInterval is how often to re-validate the session (future use).
	RevalidationInterval time.Duration

	// RevalidationEndpoint is the URL to call for re-validation (future use).
	RevalidationEndpoint string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	return &Config{
		ListenAddr:           getEnv("LISTEN_ADDR", ":8080"),
		TargetHost:           getEnv("TARGET_HOST", "127.0.0.1"),
		TargetPort:           getEnv("TARGET_PORT", "2222"),
		MaxSessionDuration:   getDurationEnv("MAX_SESSION_DURATION", 12*time.Hour),
		PingInterval:         getDurationEnv("PING_INTERVAL", 30*time.Second),
		PingTimeout:          getDurationEnv("PING_TIMEOUT", 60*time.Second),
		MaxConnections:       getIntEnv("MAX_CONNECTIONS", 10),
		RevalidationInterval: getDurationEnv("REVALIDATION_INTERVAL", 5*time.Minute),
		RevalidationEndpoint: getEnv("REVALIDATION_ENDPOINT", ""),
	}
}

// TargetAddr returns the full target address in host:port format.
func (c *Config) TargetAddr() string {
	return fmt.Sprintf("%s:%s", c.TargetHost, c.TargetPort)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return d
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return n
	}
	return defaultValue
}
