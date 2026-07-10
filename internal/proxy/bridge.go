/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	// This also bounds how long the write mutex can be held.
	writeWait = 10 * time.Second

	// tcpReadBufferSize is the size of the buffer used when reading from the TCP connection.
	// 32KB balances memory usage and syscall overhead for typical SSH traffic.
	tcpReadBufferSize = 32 * 1024
)

// Bridge handles bidirectional data copy between a WebSocket connection and a TCP connection.
// It enforces gorilla/websocket's concurrency rules: at most one concurrent reader
// and one concurrent writer. The writeMu serializes all write operations (data frames
// and control frames like ping).
type Bridge struct {
	ws      *websocket.Conn
	tcp     net.Conn
	metrics *Metrics
	logger  logr.Logger
	writeMu sync.Mutex
}

// NewBridge creates a new Bridge between a WebSocket and TCP connection.
func NewBridge(ws *websocket.Conn, tcp net.Conn, metrics *Metrics, logger logr.Logger) *Bridge {
	return &Bridge{
		ws:      ws,
		tcp:     tcp,
		metrics: metrics,
		logger:  logger,
	}
}

// Run starts bidirectional copy. It blocks until one direction errors or closes.
// Returns the first error encountered (nil if closed cleanly).
func (b *Bridge) Run() error {
	errChan := make(chan error, 2)

	// WebSocket → TCP (read pump)
	go func() {
		errChan <- b.copyWSToTCP()
	}()

	// TCP → WebSocket (write pump)
	go func() {
		errChan <- b.copyTCPToWS()
	}()

	// Wait for first error or completion
	err := <-errChan

	// Close both connections to unblock the other goroutine
	b.ws.Close()
	b.tcp.Close()

	return err
}

// WriteMessage writes a WebSocket message with proper write deadline and mutex.
func (b *Bridge) WriteMessage(messageType int, data []byte) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	b.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return b.ws.WriteMessage(messageType, data)
}

// WriteControl writes a WebSocket control frame (ping, close) with proper deadline.
// gorilla/websocket documents that WriteControl can be called concurrently with
// all other methods, but we still serialize through writeMu for safety.
func (b *Bridge) WriteControl(messageType int, data []byte, deadline time.Time) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.ws.WriteControl(messageType, data, deadline)
}

// copyWSToTCP reads WebSocket binary frames and writes them to TCP.
func (b *Bridge) copyWSToTCP() error {
	for {
		messageType, data, err := b.ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}

		if messageType != websocket.BinaryMessage {
			b.logger.V(1).Info("Ignoring non-binary message", "type", messageType)
			continue
		}

		n, err := b.tcp.Write(data)
		if err != nil {
			return err
		}

		b.metrics.BytesTransferred.WithLabelValues("ws_to_tcp").Add(float64(n))
	}
}

// copyTCPToWS reads from TCP and writes as WebSocket binary frames.
func (b *Bridge) copyTCPToWS() error {
	buf := make([]byte, tcpReadBufferSize)

	for {
		n, err := b.tcp.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := b.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			return err
		}

		b.metrics.BytesTransferred.WithLabelValues("tcp_to_ws").Add(float64(n))
	}
}
