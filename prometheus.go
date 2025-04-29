package main

import (
	"log"
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
		hostname = "png2gif web server"
	}

	s := &tsnet.Server{
		Hostname: hostname,
	}
	defer s.Close()

	ln, err := s.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on Tailscale interface: %v", err)
	}
	http.Handle("/metrics", promhttp.Handler())

	log.Println("Prometheus metrics server is running on Tailscale network at :2112")
	err = http.Serve(ln, nil)
	if err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
