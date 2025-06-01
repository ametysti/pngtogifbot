package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tailscale.com/tsnet"
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
)

func StartPrometheusHTTPHandler() {
	addr := "127.0.0.1:2112"
	hostname := "png2gif web server dev"

	if isRunningInDocker() {
		addr = ":2112"
		hostname = "png2gif web server"
	}

	s := &tsnet.Server{
		Hostname:  hostname,
		Ephemeral: true,
	}
	defer s.Close()

	s.Dial(context.Background(), "tcp", "100.64.1.0:8080")

	ln, err := s.Listen("tcp", addr)
	if err != nil {
		slog.Error("[PROMETHEUS] Failed to listen on Tailscale interface", "error", err)
	}
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("[PROMETHEUS] Prometheus metrics server is running on Tailscale network at :2112")
	err = http.Serve(ln, nil)
	if err != nil {
		slog.Error("[PROMETHEUS] Failed to open Prometheus metrics server", "error", err)
	}
}
