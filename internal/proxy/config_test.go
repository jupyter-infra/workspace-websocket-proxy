/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	config := LoadConfig()

	if config.ListenAddr != ":8080" {
		t.Errorf("expected :8080, got %s", config.ListenAddr)
	}
	if config.TargetHost != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", config.TargetHost)
	}
	if config.TargetPort != 2222 {
		t.Errorf("expected 2222, got %d", config.TargetPort)
	}
	if config.MaxSessionDuration != 12*time.Hour {
		t.Errorf("expected 12h, got %s", config.MaxSessionDuration)
	}
	if config.MaxConnections != 10 {
		t.Errorf("expected 10, got %d", config.MaxConnections)
	}
	if config.ReadLimit != 65536 {
		t.Errorf("expected 65536, got %d", config.ReadLimit)
	}
}

func TestConfigTargetAddr(t *testing.T) {
	config := &Config{TargetHost: "localhost", TargetPort: 3333}
	expected := "localhost:3333"
	if config.TargetAddr() != expected {
		t.Errorf("expected %s, got %s", expected, config.TargetAddr())
	}
}
