/*
Copyright (c) Amazon Web Services
Distributed under the terms of the MIT license
*/

package proxy

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the proxy.
type Metrics struct {
	ConnectionsTotal   prometheus.Counter
	ActiveConnections  prometheus.Gauge
	ConnectionErrors   *prometheus.CounterVec
	BytesTransferred   *prometheus.CounterVec
	ConnectionDuration prometheus.Histogram
	Registry           *prometheus.Registry
}

// NewMetrics creates and registers all proxy metrics with a dedicated registry.
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	connectionsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ws_proxy_connections_total",
		Help: "Total number of WebSocket connections established",
	})

	activeConnections := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ws_proxy_active_connections",
		Help: "Number of currently active WebSocket connections",
	})

	connectionErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_proxy_connection_errors_total",
			Help: "Total connection errors by type",
		},
		[]string{"reason"},
	)

	bytesTransferred := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_proxy_bytes_transferred_total",
			Help: "Total bytes transferred by direction",
		},
		[]string{"direction"},
	)

	connectionDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ws_proxy_connection_duration_seconds",
		Help:    "Connection duration in seconds",
		Buckets: []float64{1, 10, 60, 300, 600, 1800, 3600, 7200, 14400, 28800, 43200},
	})

	registry.MustRegister(connectionsTotal, activeConnections, connectionErrors, bytesTransferred, connectionDuration)

	return &Metrics{
		ConnectionsTotal:   connectionsTotal,
		ActiveConnections:  activeConnections,
		ConnectionErrors:   connectionErrors,
		BytesTransferred:   bytesTransferred,
		ConnectionDuration: connectionDuration,
		Registry:           registry,
	}
}
