/*
Copyright (c) 2026 Jupyter Infrastructure
Distributed under the terms of the MIT license
*/

package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/gorilla/websocket"
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
		TargetPort:         "0",
		MaxSessionDuration: 5 * time.Second,
		PingInterval:       1 * time.Second,
		PingTimeout:        2 * time.Second,
		MaxConnections:     2,
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

	return listener.Addr().String(), func() { listener.Close() }
}

func TestHealthEndpoint(t *testing.T) {
	config := testConfig()
	server := NewServer(config, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("expected body to contain 'ok', got %s", w.Body.String())
	}
}

func TestWebSocketBridge(t *testing.T) {
	// Start echo TCP server
	tcpAddr, cleanupTCP := startEchoTCPServer(t)
	defer cleanupTCP()

	// Parse port from address
	_, port, _ := net.SplitHostPort(tcpAddr)

	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = port

	server := NewServer(config, testLogger())

	// Start the proxy server
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	// Send test data
	testData := []byte("hello websocket proxy")
	if err := ws.WriteMessage(websocket.BinaryMessage, testData); err != nil {
		t.Fatal(err)
	}

	// Read echo response
	_, response, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}

	if string(response) != string(testData) {
		t.Errorf("expected %q, got %q", testData, response)
	}
}

func TestMaxConnections(t *testing.T) {
	tcpAddr, cleanupTCP := startEchoTCPServer(t)
	defer cleanupTCP()

	_, port, _ := net.SplitHostPort(tcpAddr)

	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = port
	config.MaxConnections = 1

	server := NewServer(config, testLogger())
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// First connection should succeed
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws1.Close()

	// Give the session a moment to register
	time.Sleep(50 * time.Millisecond)

	// Second connection should be rejected with 503
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected second connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}
}

func TestMaxSessionDuration(t *testing.T) {
	tcpAddr, cleanupTCP := startEchoTCPServer(t)
	defer cleanupTCP()

	_, port, _ := net.SplitHostPort(tcpAddr)

	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = port
	config.MaxSessionDuration = 500 * time.Millisecond
	config.PingInterval = 100 * time.Millisecond
	config.PingTimeout = 1 * time.Second

	server := NewServer(config, testLogger())
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	// Connection should be closed after max duration
	time.Sleep(800 * time.Millisecond)

	// Attempt to write should fail
	err = ws.WriteMessage(websocket.BinaryMessage, []byte("test"))
	if err == nil {
		// Try reading — it should fail
		_, _, err = ws.ReadMessage()
		if err == nil {
			t.Error("expected connection to be closed after max duration")
		}
	}
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager(3)

	// Acquire up to limit
	if !sm.Acquire() {
		t.Error("expected first acquire to succeed")
	}
	if !sm.Acquire() {
		t.Error("expected second acquire to succeed")
	}
	if !sm.Acquire() {
		t.Error("expected third acquire to succeed")
	}

	// Fourth should fail
	if sm.Acquire() {
		t.Error("expected fourth acquire to fail")
	}

	if sm.ActiveCount() != 3 {
		t.Errorf("expected 3 active, got %d", sm.ActiveCount())
	}

	// Release one
	sm.Release()

	if sm.ActiveCount() != 2 {
		t.Errorf("expected 2 active, got %d", sm.ActiveCount())
	}

	// Should succeed again
	if !sm.Acquire() {
		t.Error("expected acquire after release to succeed")
	}
}

func TestConfigDefaults(t *testing.T) {
	config := LoadConfig()

	if config.ListenAddr != ":8080" {
		t.Errorf("expected :8080, got %s", config.ListenAddr)
	}
	if config.TargetHost != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", config.TargetHost)
	}
	if config.TargetPort != "2222" {
		t.Errorf("expected 2222, got %s", config.TargetPort)
	}
	if config.MaxSessionDuration != 12*time.Hour {
		t.Errorf("expected 12h, got %s", config.MaxSessionDuration)
	}
	if config.MaxConnections != 10 {
		t.Errorf("expected 10, got %d", config.MaxConnections)
	}
}

func TestConfigTargetAddr(t *testing.T) {
	config := &Config{TargetHost: "localhost", TargetPort: "3333"}
	expected := "localhost:3333"
	if config.TargetAddr() != expected {
		t.Errorf("expected %s, got %s", expected, config.TargetAddr())
	}
}

func TestNoOpRevalidator(t *testing.T) {
	r := &NoOpRevalidator{}
	if err := r.Revalidate(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestTCPDialFailure(t *testing.T) {
	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = "1" // Port 1 should be unreachable

	server := NewServer(config, testLogger())
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// Connection might be rejected immediately — that's fine
		return
	}
	defer ws.Close()

	// Set a read deadline so the test doesn't hang
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	// The WebSocket should close quickly since TCP dial fails
	_, _, err = ws.ReadMessage()
	if err == nil {
		t.Error("expected read to fail after TCP dial failure")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	config := testConfig()
	server := NewServer(config, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ws_proxy") {
		t.Errorf("expected metrics output to contain ws_proxy prefix")
	}
}
