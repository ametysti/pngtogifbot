package main

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	discordLatency = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "discord_gateway_latency_seconds",
			Help: "Current latency to Discord's gateway in seconds.",
		},
		[]string{"bot"},
	)

	discordGuildCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "discord_bot_guild_count",
			Help: "guild count",
		},
		[]string{"bot"},
	)

	uploadCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uploads_total",
		Help: "The total number of successful uploads",
	})

	backupUploadFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "backup_upload_failures_total",
			Help: "Number of failed backup uploads by error type",
		},
		[]string{"reason"},
	)

	uploadFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "upload_failures_total",
		Help: "Number of regular upload failures by type",
	}, []string{"reason"})

	totalFilesGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "storage_total_files",
		Help: "Total number of files in storage zone",
	})
	totalSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "storage_total_size_bytes",
		Help: "Total size in bytes of all files in storage zone",
	})
	discordConnectionEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "discord_connection_events_total",
			Help: "Number of Discord connection events by type (e.g., ready, disconnect, resume)",
		},
		[]string{"bot", "event"},
	)
	discordConnectionStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "discord_connection_status",
		Help: "Current Discord connection status (1 = connected, 0 = disconnected)",
	})
)

func StartPrometheusHTTPHandler() {
	addr := "127.0.0.1:2112"

	if isRunningInDocker() {
		addr = ":2112"
	}

	http.Handle("/metrics", promhttp.Handler())

	slog.Info("[PROMETHEUS] Starting Prometheus metrics server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("[PROMETHEUS] Failed to start HTTP server", "error", err)
	}
}
