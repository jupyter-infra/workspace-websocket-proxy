/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

func testLogger() logr.Logger {
	zapLog, _ := zap.NewDevelopment()
	return zapr.NewLogger(zapLog)
}

func testConfig() *Config {
	return &Config{
		ListenAddr:         ":0",
		TargetHost:         "127.0.0.1",
		TargetPort:         0,
		MaxSessionDuration: 5 * time.Second,
		PingInterval:       1 * time.Second,
		PingTimeout:        2 * time.Second,
		MaxConnections:     2,
		ReadLimit:          65536,
	}
}

// startEchoTCPServer starts a TCP server that echoes received data back.
func startEchoTCPServer(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	return listener.Addr().String(), func() { _ = listener.Close() }
}
