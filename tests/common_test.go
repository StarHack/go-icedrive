package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StarHack/go-icedrive/api"
)

var (
	testEmail          = os.Getenv("ICEDRIVE_TEST_EMAIL")
	testPassword       = os.Getenv("ICEDRIVE_TEST_PASSWORD")
	testCryptoPassword = os.Getenv("ICEDRIVE_TEST_CRYPTO_PASSWORD")
)

func skipIfNoCredentials(t *testing.T) {
	if testEmail == "" || testPassword == "" {
		t.Skip("Skipping test: ICEDRIVE_TEST_EMAIL and ICEDRIVE_TEST_PASSWORD not set")
	}
}

func skipIfNoCryptoPassword(t *testing.T) {
	skipIfNoCredentials(t)
	if testCryptoPassword == "" {
		t.Skip("Skipping test: ICEDRIVE_TEST_CRYPTO_PASSWORD not set")
	}
}

func generateTestFile(t *testing.T, size int) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, fmt.Sprintf("test_%d.bin", time.Now().Unix()))

	content := make([]byte, size)
	for i := 0; i < size; i++ {
		content[i] = byte(i % 256)
	}

	if err := os.WriteFile(filename, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	t.Logf("Generated test file: %s (size: %d bytes, hash: %s)", filename, size, hashStr[:16]+"...")
	return filename, hashStr
}

func findItemByName(items []api.Item, name string) *api.Item {
	for i := range items {
		if items[i].Filename == name {
			return &items[i]
		}
	}
	return nil
}

func verifyFileHash(t *testing.T, filePath, expectedHash string) {
	t.Helper()

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file for verification: %v", err)
	}

	hash := sha256.Sum256(data)
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		t.Errorf("File integrity check failed:\nExpected: %s\nActual:   %s", expectedHash, actualHash)
	} else {
		t.Logf("✓ File integrity verified (hash: %s...)", actualHash[:16])
	}
}
