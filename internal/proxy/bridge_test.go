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

	"github.com/gorilla/websocket"
)

func TestWebSocketBridge(t *testing.T) {
	tcpAddr, cleanupTCP := startEchoTCPServer(t)
	defer cleanupTCP()

	_, port, _ := net.SplitHostPort(tcpAddr)

	config := testConfig()
	config.TargetHost = "127.0.0.1"
	config.TargetPort = port

	server := NewServer(config, testLogger())
	ts := httptest.NewServer(server.httpServer.Handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	testData := []byte("hello websocket proxy")
	if err := ws.WriteMessage(websocket.BinaryMessage, testData); err != nil {
		t.Fatal(err)
	}

	_, response, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}

	if string(response) != string(testData) {
		t.Errorf("expected %q, got %q", testData, response)
	}
}
