package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StarHack/go-icedrive/client"
)

func TestUnencryptedFileUploadWorkflow(t *testing.T) {
	skipIfNoCredentials(t)

	c := client.NewClient()
	c.SetDebug(false)

	t.Log("Step 1: Login")
	if err := c.LoginWithUsernameAndPassword(testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	testSize := 256 * 1024
	testFilePath, originalHash := generateTestFile(t, testSize)
	testFileName := filepath.Base(testFilePath)
	defer os.Remove(testFilePath)

	t.Log("Step 2: Upload unencrypted file via streaming")
	writer, err := c.UploadFileWriter(0, testFileName)
	if err != nil {
		t.Fatalf("Failed to create upload writer: %v", err)
	}

	file, err := os.Open(testFilePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	buf := make([]byte, 64*1024)
	written, err := io.CopyBuffer(writer, file, buf)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
	t.Logf("Wrote %d bytes", written)

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close upload writer: %v", err)
	}
	t.Logf("✓ Uploaded unencrypted file: %s", testFileName)

	time.Sleep(2 * time.Second)

	t.Log("Step 3: List folder")
	items, err := c.ListFolder(0)
	if err != nil {
		t.Fatalf("Failed to list folder: %v", err)
	}

	uploadedItem := findItemByName(items, testFileName)
	if uploadedItem == nil {
		t.Fatalf("Uploaded file not found in folder listing")
	}
	t.Logf("✓ Found uploaded file (UID: %s)", uploadedItem.UID)

	t.Log("Step 4: Rename file")
	renamedFileName := fmt.Sprintf("renamed_plain_%d.bin", time.Now().Unix())
	if err := c.Rename(*uploadedItem, renamedFileName); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}
	t.Logf("✓ Renamed to: %s", renamedFileName)

	time.Sleep(1 * time.Second)
	items, err = c.ListFolder(0)
	if err != nil {
		t.Fatalf("Failed to list folder after rename: %v", err)
	}
	renamedItem := findItemByName(items, renamedFileName)
	if renamedItem == nil {
		t.Fatalf("Renamed file not found in folder listing")
	}

	t.Log("Step 5: Create test directory")
	testDirName := fmt.Sprintf("test_plain_dir_%d", time.Now().Unix())
	if err := c.CreateFolder(0, testDirName); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	t.Logf("✓ Created directory: %s", testDirName)

	time.Sleep(1 * time.Second)
	items, err = c.ListFolder(0)
	if err != nil {
		t.Fatalf("Failed to list folder after directory creation: %v", err)
	}
	testDir := findItemByName(items, testDirName)
	if testDir == nil {
		t.Fatalf("Created directory not found in folder listing")
	}
	t.Logf("✓ Found directory (UID: %s)", testDir.UID)

	t.Log("Step 6: Move file to test directory")
	var testDirID uint64
	_, err = fmt.Sscanf(testDir.UID, "folder-%d", &testDirID)
	if err != nil || testDirID == 0 {
		t.Fatalf("Failed to parse directory ID from UID: %s (error: %v)", testDir.UID, err)
	}

	if err := c.Move(testDirID, *renamedItem); err != nil {
		t.Fatalf("Failed to move file: %v", err)
	}
	t.Logf("✓ Moved file to directory (ID: %d)", testDirID)

	time.Sleep(1 * time.Second)
	dirItems, err := c.ListFolder(testDirID)
	if err != nil {
		t.Fatalf("Failed to list test directory: %v", err)
	}
	movedItem := findItemByName(dirItems, renamedFileName)
	if movedItem == nil {
		t.Fatalf("File not found in test directory after move")
	}
	t.Logf("✓ Verified file in test directory")

	t.Log("Step 7: Download file via streaming and verify integrity")
	downloadStream, err := c.DownloadFileStream(*movedItem)
	if err != nil {
		t.Fatalf("Failed to open download stream: %v", err)
	}
	defer downloadStream.Close()

	hasher := sha256.New()
	downloadedBytes, err := io.Copy(hasher, downloadStream)
	if err != nil {
		t.Fatalf("Failed to read download stream: %v", err)
	}
	t.Logf("Downloaded %d bytes", downloadedBytes)

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != originalHash {
		t.Errorf("File integrity check failed:\nExpected: %s\nActual:   %s", originalHash, actualHash)
	} else {
		t.Logf("✓ File integrity verified via streaming (hash: %s...)", actualHash[:16])
	}

	t.Log("Step 8: Test non-streaming download")
	downloadDir := t.TempDir()
	if err := c.DownloadFile(*movedItem, downloadDir); err != nil {
		t.Fatalf("Failed to download file (non-streaming): %v", err)
	}
	downloadedPath := filepath.Join(downloadDir, renamedFileName)
	verifyFileHash(t, downloadedPath, originalHash)

	t.Log("Cleanup: Deleting test directory")
	if err := c.Delete(*testDir); err != nil {
		t.Logf("Warning: Failed to delete test directory: %v", err)
	} else {
		t.Logf("✓ Cleaned up test directory")
	}

	t.Log("✅ Unencrypted file upload workflow test completed successfully")
}

func TestStreamingIntegrity(t *testing.T) {
	skipIfNoCredentials(t)

	c := client.NewClient()
	c.SetDebug(false)

	if err := c.LoginWithUsernameAndPassword(testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	sizes := []int{
		1024,
		64 * 1024,
		1024 * 1024,
		5 * 1024 * 1024,
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Size_%dKB", size/1024), func(t *testing.T) {
			content := make([]byte, size)
			for i := 0; i < size; i++ {
				content[i] = byte(i % 256)
			}
			originalHash := sha256.Sum256(content)

			testName := fmt.Sprintf("stream_test_%d_%d.bin", size, time.Now().Unix())
			writer, err := c.UploadFileWriter(0, testName)
			if err != nil {
				t.Fatalf("Failed to create upload writer: %v", err)
			}

			reader := bytes.NewReader(content)
			if _, err := io.Copy(writer, reader); err != nil {
				t.Fatalf("Failed to upload: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Failed to close writer: %v", err)
			}

			time.Sleep(2 * time.Second)

			items, err := c.ListFolder(0)
			if err != nil {
				t.Fatalf("Failed to list folder: %v", err)
			}
			item := findItemByName(items, testName)
			if item == nil {
				t.Fatalf("Uploaded file not found")
			}

			downloadStream, err := c.DownloadFileStream(*item)
			if err != nil {
				t.Fatalf("Failed to download: %v", err)
			}
			defer downloadStream.Close()

			hasher := sha256.New()
			if _, err := io.Copy(hasher, downloadStream); err != nil {
				t.Fatalf("Failed to read download: %v", err)
			}

			downloadedHash := hasher.Sum(nil)
			if !bytes.Equal(originalHash[:], downloadedHash) {
				t.Errorf("Hash mismatch for size %d", size)
			} else {
				t.Logf("✓ Integrity verified for %d KB", size/1024)
			}

			c.Delete(*item)
		})
	}
}
