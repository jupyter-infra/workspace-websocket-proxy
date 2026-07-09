/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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

	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws1.Close()

	time.Sleep(50 * time.Millisecond)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected second connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}
}

func TestTCPDialFailure(t *testing.T) {
	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = "1"

	server := NewServer(config, testLogger())
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	_, _, err = ws.ReadMessage()
	if err == nil {
		t.Error("expected read to fail after TCP dial failure")
	}
}
