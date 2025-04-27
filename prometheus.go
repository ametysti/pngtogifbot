package main

import (
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
)

func StartPrometheusHTTPHandler() {
	http.Handle("/metrics", promhttp.Handler())

	addr := "127.0.0.1:2112"

	if isRunningInDocker() {
		addr = ":2112"
	}

	http.ListenAndServe(addr, nil)
}
