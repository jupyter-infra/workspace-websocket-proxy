/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

const (
	// closeGracePeriod is the time to wait after sending a close frame
	// before force-closing the connection.
	closeGracePeriod = 5 * time.Second
)

// SessionManager tracks active sessions and enforces concurrency limits.
type SessionManager struct {
	maxConnections int32
	active         atomic.Int32
	mu             sync.Mutex
}

// NewSessionManager creates a new SessionManager with the given concurrency limit.
func NewSessionManager(maxConnections int) *SessionManager {
	return &SessionManager{
		maxConnections: int32(maxConnections),
	}
}

// Acquire attempts to acquire a session slot. Returns false if at capacity.
func (sm *SessionManager) Acquire() bool {
	for {
		current := sm.active.Load()
		if current >= sm.maxConnections {
			return false
		}
		if sm.active.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// Release releases a session slot.
func (sm *SessionManager) Release() {
	sm.active.Add(-1)
}

// ActiveCount returns the current number of active sessions.
func (sm *SessionManager) ActiveCount() int32 {
	return sm.active.Load()
}

// Session represents a single proxied connection with lifecycle management.
type Session struct {
	ws          *websocket.Conn
	config      *Config
	metrics     *Metrics
	logger      logr.Logger
	revalidator Revalidator
	cancel      context.CancelFunc
}

// NewSession creates a new session for a WebSocket connection.
func NewSession(ws *websocket.Conn, config *Config, metrics *Metrics, logger logr.Logger, revalidator Revalidator) *Session {
	return &Session{
		ws:          ws,
		config:      config,
		metrics:     metrics,
		logger:      logger,
		revalidator: revalidator,
	}
}

// Run manages the full lifecycle of the session: sets up ping/pong, enforces
// max duration, and runs the bridge. Blocks until the session ends.
func (s *Session) Run(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		s.metrics.ConnectionDuration.Observe(duration)
	}()

	// Configure pong handler: reset read deadline on each pong received.
	// This follows the gorilla/websocket recommended pattern from examples/chat.
	s.ws.SetPongHandler(func(string) error {
		return s.ws.SetReadDeadline(time.Now().Add(s.config.PingTimeout))
	})

	// Set initial read deadline — if no pong arrives within PingTimeout, reads will fail.
	if err := s.ws.SetReadDeadline(time.Now().Add(s.config.PingTimeout)); err != nil {
		return err
	}

	// Dial the TCP target
	tcp, err := dialTCP(ctx, s.config.TargetAddr())
	if err != nil {
		s.logger.Error(err, "Failed to dial TCP target", "addr", s.config.TargetAddr())
		s.metrics.ConnectionErrors.WithLabelValues("tcp_dial_failed").Inc()
		return err
	}
	defer tcp.Close()

	s.logger.Info("Session established", "target", s.config.TargetAddr())

	// Create bridge (holds the write mutex for safe concurrent writes)
	bridge := NewBridge(s.ws, tcp, s.metrics, s.logger)

	// Start ping ticker — uses bridge.WriteControl for thread-safe writes.
	// Ping period must be less than pong timeout (gorilla best practice).
	pingDone := make(chan struct{})
	go s.pingLoop(ctx, bridge, pingDone)
	defer func() { <-pingDone }()

	// Start max duration timer
	maxDurationTimer := time.AfterFunc(s.config.MaxSessionDuration, func() {
		s.logger.Info("Max session duration reached, closing connection",
			"duration", s.config.MaxSessionDuration)
		s.metrics.ConnectionErrors.WithLabelValues("max_duration").Inc()
		// Send close frame then cancel context after grace period
		bridge.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session duration limit reached"),
			time.Now().Add(writeWait),
		)
		// Give the peer time to process the close frame
		time.AfterFunc(closeGracePeriod, func() {
			s.cancel()
		})
	})
	defer maxDurationTimer.Stop()

	// Run bridge in a goroutine so we can select on context cancellation
	bridgeErr := make(chan error, 1)
	go func() {
		bridgeErr <- bridge.Run()
	}()

	select {
	case err := <-bridgeErr:
		return err
	case <-ctx.Done():
		// Context cancelled (max duration or external termination)
		s.ws.Close()
		tcp.Close()
		return ctx.Err()
	}
}

// pingLoop sends periodic WebSocket pings using the bridge's write mutex.
// Follows gorilla best practice: ping period < pong timeout.
func (s *Session) pingLoop(ctx context.Context, bridge *Bridge, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bridge.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				s.logger.Error(err, "Ping failed, closing connection")
				s.metrics.ConnectionErrors.WithLabelValues("ping_failed").Inc()
				s.cancel()
				return
			}
		}
	}
}
