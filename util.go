package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"

func GenerateRandomString(length int) (string, error) {
	var result []byte
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		randIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random index: %v", err)
		}
		result = append(result, charset[randIndex.Int64()])
	}

	return string(result), nil
}

func isRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	return false
}
