/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/jupyter-infra/workspace-websocket-proxy/internal/proxy"
	"go.uber.org/zap"
)

func main() {
	// Health check mode for Kubernetes exec probes.
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		port := os.Getenv("LISTEN_ADDR")
		if port == "" {
			port = ":8080"
		}
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1%s/health", port))
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Setup logger
	zapLog, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	logger := zapr.NewLogger(zapLog).WithName("ws-proxy")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	_ = logr.NewContext(ctx, logger)

	// Load configuration
	config := proxy.LoadConfig()

	// Create and start server
	server := proxy.NewServer(config, logger)

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Info("Shutting down WebSocket proxy")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "Server shutdown failed")
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil {
			logger.Error(err, "Server exited with error")
			os.Exit(1)
		}
	}

	logger.Info("Server stopped")
}
