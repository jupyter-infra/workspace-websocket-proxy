/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the main WebSocket proxy HTTP server.
type Server struct {
	config         *Config
	metrics        *Metrics
	sessionManager *SessionManager
	revalidator    Revalidator
	logger         logr.Logger
	httpServer     *http.Server
}

// NewServer creates a new proxy Server.
func NewServer(config *Config, logger logr.Logger) *Server {
	s := &Server{
		config:         config,
		metrics:        NewMetrics(),
		sessionManager: NewSessionManager(config.MaxConnections),
		revalidator:    &NoOpRevalidator{},
		logger:         logger.WithName("server"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.Handle("/metrics", promhttp.HandlerFor(s.metrics.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:    config.ListenAddr,
		Handler: mux,
	}

	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	s.logger.Info("Starting WebSocket proxy",
		"addr", s.config.ListenAddr,
		"target", s.config.TargetAddr(),
		"maxConnections", s.config.MaxConnections,
		"maxSessionDuration", s.config.MaxSessionDuration,
		"pingInterval", s.config.PingInterval,
		"pingTimeout", s.config.PingTimeout,
	)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handleHealth responds with 200 OK for Kubernetes probes.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","activeConnections":%d}`, s.sessionManager.ActiveCount())
}

// upgrader configures the WebSocket upgrade.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Origin check is handled by Traefik ForwardAuth before traffic reaches us.
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// handleWebSocket upgrades the HTTP connection and starts a proxied session.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.WithValues("remoteAddr", r.RemoteAddr)

	// Enforce concurrency limit
	if !s.sessionManager.Acquire() {
		logger.Info("Connection rejected: at capacity",
			"active", s.sessionManager.ActiveCount(),
			"max", s.config.MaxConnections)
		s.metrics.ConnectionErrors.WithLabelValues("capacity_exceeded").Inc()
		http.Error(w, "Service at capacity", http.StatusServiceUnavailable)
		return
	}
	defer s.sessionManager.Release()

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error(err, "WebSocket upgrade failed")
		s.metrics.ConnectionErrors.WithLabelValues("upgrade_failed").Inc()
		return
	}
	defer ws.Close()

	s.metrics.ConnectionsTotal.Inc()
	s.metrics.ActiveConnections.Inc()
	defer s.metrics.ActiveConnections.Dec()

	logger.Info("WebSocket connection established")

	// Create and run session
	session := NewSession(ws, s.config, s.metrics, logger, s.revalidator)
	if err := session.Run(r.Context()); err != nil {
		if err != context.Canceled {
			logger.V(1).Info("Session ended with error", "error", err)
		}
	}

	logger.Info("WebSocket connection closed")
}

// dialTCP establishes a TCP connection to the target with timeout.
func dialTCP(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	return dialer.DialContext(ctx, "tcp", addr)
}
