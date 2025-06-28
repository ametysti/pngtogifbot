package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func UploadToBackupSite(ctx context.Context, filepath string, filename string, body io.ReadSeeker) {
	body.Seek(0, io.SeekStart)

	filepath = strings.ReplaceAll(filepath, "/", "")

	key := fmt.Sprintf("%s/%s", filepath, filename)

	result, err := S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("png2gif-files"),
		Key:    aws.String(key),
		Body:   body,
	})

	if err != nil {
		slog.Error("[S3] Failed to upload to backup site", "file", key, "error", err)
		return
	}

	slog.Info("[S3] Uploaded file to backup site", "key", key, "etag", aws.ToString(result.ETag))
}
