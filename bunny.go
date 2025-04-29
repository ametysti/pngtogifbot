package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

type PullZoneStats struct {
	TotalBandwidthUsed        int64              `json:"TotalBandwidthUsed"`
	TotalOriginTraffic        int64              `json:"TotalOriginTraffic"`
	AverageOriginResponseTime float64            `json:"AverageOriginResponseTime"`
	OriginResponseTimeChart   map[string]float64 `json:"OriginResponseTimeChart"`
	TotalRequestsServed       int64              `json:"TotalRequestsServed"`
	CacheHitRate              float64            `json:"CacheHitRate"`
	BandwidthUsedChart        map[string]float64 `json:"BandwidthUsedChart"`
	BandwidthCachedChart      map[string]float64 `json:"BandwidthCachedChart"`
	CacheHitRateChart         map[string]float64 `json:"CacheHitRateChart"`
	RequestsServedChart       map[string]float64 `json:"RequestsServedChart"`
	PullRequestsPulledChart   map[string]float64 `json:"PullRequestsPulledChart"`
}

type StorageStats struct {
	StorageUsedChart map[string]int `json:"StorageUsedChart"`
	FileCountChart   map[string]int `json:"FileCountChart"`
}

type LatestChartData struct {
	StorageUsed int `json:"latestStorageUsed"`
	FileCount   int `json:"latestFileCount"`
}

func GetPullZoneStats() (PullZoneStats, error) {
	url := "https://api.bunny.net/statistics?pullZone=3680182"

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")

	req.Header.Add("AccessKey", os.Getenv("BUNNYNET_API_KEY"))

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return PullZoneStats{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return PullZoneStats{}, err
	}

	var stats PullZoneStats

	err = json.Unmarshal(body, &stats)

	if err != nil {
		return PullZoneStats{}, err
	}

	return stats, nil
}

func GetStorageZoneStats() (LatestChartData, error) {
	url := fmt.Sprintf(
		"https://api.bunny.net/storagezone/999741/statistics?dateFrom=%s&dateTo=%s",
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("accept", "application/json")
	req.Header.Add("AccessKey", os.Getenv("BUNNYNET_API_KEY"))

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return LatestChartData{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return LatestChartData{}, err
	}

	var stats StorageStats
	err = json.Unmarshal(body, &stats)
	if err != nil {
		return LatestChartData{}, err
	}

	var storageTimes []string
	for t := range stats.StorageUsedChart {
		storageTimes = append(storageTimes, t)
	}
	sort.Strings(storageTimes)

	var fileTimes []string
	for t := range stats.FileCountChart {
		fileTimes = append(fileTimes, t)
	}
	sort.Strings(fileTimes)

	var latestStorageUsed, latestFileCount int
	if len(storageTimes) > 0 {
		latestStorageUsed = stats.StorageUsedChart[storageTimes[len(storageTimes)-1]]
	}
	if len(fileTimes) > 0 {
		latestFileCount = stats.FileCountChart[fileTimes[len(fileTimes)-1]]
	}

	return LatestChartData{
		StorageUsed: latestStorageUsed,
		FileCount:   latestFileCount,
	}, nil
}
