package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

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

func Upload(ctx context.Context, filepath string, filename string, checksum string, body io.Reader) (*Response, error) {
	StorageZoneName := os.Getenv("BUNNYNET_CDN_STORAGE_NAME")
	AccessKey := os.Getenv("BUNNYNET_CDN_STORAGE_KEY")
	Region := os.Getenv("BUNNYNET_CDN_STORAGE_REGION")

	baseURL := "storage.bunnycdn.com"
	if Region != "" {
		baseURL = Region + "." + baseURL
	}

	url := fmt.Sprintf("https://%s%s", baseURL, path.Join("/", StorageZoneName, filepath, filename))

	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
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
		slog.Error("Failed updating avatar")
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

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
