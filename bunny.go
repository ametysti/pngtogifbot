package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

type BunnyStorageObject struct {
	ObjectName  string `json:"ObjectName"`
	Length      int64  `json:"Length"`
	IsDirectory bool   `json:"IsDirectory"`
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

func GetStorageZoneStats() (int, int64, error) {
	url := "https://storage.bunnycdn.com/pngtogif/gifs/"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("creating request failed: %w", err)
	}
	req.Header.Set("AccessKey", os.Getenv("BUNNYNET_CDN_STORAGE_KEY"))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("reading response body failed: %w", err)
	}

	var objects []BunnyStorageObject
	if err := json.Unmarshal(body, &objects); err != nil {
		return 0, 0, fmt.Errorf("unmarshaling response failed: %w", err)
	}

	var totalFiles int
	var totalSize int64

	for _, obj := range objects {
		if obj.IsDirectory {
			continue
		}
		totalFiles++
		totalSize += int64(obj.Length)
	}

	return totalFiles, totalSize, nil
}
