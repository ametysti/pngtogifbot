package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"time"

	"net/http"
)

type Response struct {
	Header http.Header
	Body   []byte
	Status int
}

type Object struct {
	UserID          string `json:"UserId,omitempty"`
	ContentType     string `json:"ContentType,omitempty"`
	Path            string `json:"Path,omitempty"`
	ObjectName      string `json:"ObjectName,omitempty"`
	ReplicatedZones string `json:"ReplicatedZones,omitempty"`
	LastChanged     string `json:"LastChanged,omitempty"`
	StorageZoneName string `json:"StorageZoneName,omitempty"`
	Checksum        string `json:"Checksum,omitempty"`
	DateCreated     string `json:"DateCreated,omitempty"`
	GUID            string `json:"Guid,omitempty"`
	Length          int    `json:"Length,omitempty"`
	ServerID        int    `json:"ServerId,omitempty"`
	StorageZoneID   int    `json:"StorageZoneId,omitempty"`
	ArrayNumber     int    `json:"ArrayNumber,omitempty"`
	IsDirectory     bool   `json:"IsDirectory,omitempty"`
}

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

func Upload(ctx context.Context, filepath string, filename string, checksum string, body io.Reader) (*Response, error) {
	StorageZoneName := os.Getenv("BUNNYNET_CDN_STORAGE_NAME")
	AccessKey := os.Getenv("BUNNYNET_CDN_STORAGE_KEY")
	Region := os.Getenv("BUNNYNET_CDN_STORAGE_REGION")

	baseURL := "storage.bunnycdn.com"
	if Region != "" {
		baseURL = Region + "." + baseURL
	}

	url := fmt.Sprintf("https://%s%s", baseURL, path.Join("/", StorageZoneName, filepath, filename))

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	bodyForReq := bytes.NewReader(data)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bodyForReq)
	if err != nil {
		return nil, err
	}

	req.Header.Set("AccessKey", AccessKey)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")

	if checksum != "" {
		req.Header.Set("Checksum", checksum)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed updating avatar", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	go UploadToBackupSite(ctx, filepath, filename, bytes.NewReader(data))

	return &Response{
		Status: resp.StatusCode,
		Body:   responseBody,
	}, nil
}

func Delete(ctx context.Context, filepath string, filename string) (*Response, error) {
	StorageZoneName := os.Getenv("BUNNYNET_CDN_STORAGE_NAME")
	AccessKey := os.Getenv("BUNNYNET_CDN_STORAGE_KEY")
	Region := os.Getenv("BUNNYNET_CDN_STORAGE_REGION")

	baseURL := "storage.bunnycdn.com"
	if Region != "" {
		baseURL = Region + "." + baseURL
	}

	url := fmt.Sprintf("https://%s%s", baseURL, path.Join("/", StorageZoneName, filepath, filename))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("AccessKey", AccessKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	return &Response{
		Status: resp.StatusCode,
		Body:   responseBody,
	}, nil
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
