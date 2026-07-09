/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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

	time.Sleep(800 * time.Millisecond)

	err = ws.WriteMessage(websocket.BinaryMessage, []byte("test"))
	if err == nil {
		_, _, err = ws.ReadMessage()
		if err == nil {
			t.Error("expected connection to be closed after max duration")
		}
	}
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager(3)

	if !sm.Acquire() {
		t.Error("expected first acquire to succeed")
	}
	if !sm.Acquire() {
		t.Error("expected second acquire to succeed")
	}
	if !sm.Acquire() {
		t.Error("expected third acquire to succeed")
	}

	if sm.Acquire() {
		t.Error("expected fourth acquire to fail")
	}

	if sm.ActiveCount() != 3 {
		t.Errorf("expected 3 active, got %d", sm.ActiveCount())
	}

	sm.Release()

	if sm.ActiveCount() != 2 {
		t.Errorf("expected 2 active, got %d", sm.ActiveCount())
	}

	if !sm.Acquire() {
		t.Error("expected acquire after release to succeed")
	}
}
