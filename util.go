package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"

// GenerateRandomString generates a secure random string of the given length.
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
