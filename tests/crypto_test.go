package tests

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StarHack/go-icedrive/client"
)

func TestEncryptedFileUploadWorkflow(t *testing.T) {
	skipIfNoCryptoPassword(t)

	c := client.NewClient()
	c.SetDebug(false)

	t.Log("Step 1: Login")
	if err := c.LoginWithUsernameAndPassword(testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	c.SetCryptoPassword(testCryptoPassword)
	saltDisplay := c.CryptoSalt
	if len(saltDisplay) > 16 {
		saltDisplay = saltDisplay[:16] + "..."
	}
	t.Logf("Crypto key set (salt: %s)", saltDisplay)

	testSize := 512 * 1024
	testFilePath, originalHash := generateTestFile(t, testSize)
	testFileName := filepath.Base(testFilePath)
	defer os.Remove(testFilePath)

	t.Log("Step 2: Upload encrypted file via streaming")
	writer, err := c.UploadFileEncryptedWriter(0, testFileName)
	if err != nil {
		t.Fatalf("Failed to create upload writer: %v", err)
	}

	file, err := os.Open(testFilePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	written, err := io.Copy(writer, file)
	file.Close()
	if err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
	t.Logf("Wrote %d bytes", written)

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close upload writer: %v", err)
	}
	t.Logf("✓ Uploaded encrypted file: %s", testFileName)

	time.Sleep(2 * time.Second)

	t.Log("Step 3: List encrypted folder")
	items, err := c.ListFolderEncrypted(0)
	if err != nil {
		t.Fatalf("Failed to list folder: %v", err)
	}

	uploadedItem := findItemByName(items, testFileName)
	if uploadedItem == nil {
		t.Fatalf("Uploaded file not found in folder listing")
	}
	t.Logf("✓ Found uploaded file (UID: %s)", uploadedItem.UID)

	t.Log("Step 4: Rename file")
	renamedFileName := fmt.Sprintf("renamed_%d.bin", time.Now().Unix())
	if err := c.Rename(*uploadedItem, renamedFileName); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}
	t.Logf("✓ Renamed to: %s", renamedFileName)

	time.Sleep(1 * time.Second)
	items, err = c.ListFolderEncrypted(0)
	if err != nil {
		t.Fatalf("Failed to list folder after rename: %v", err)
	}
	renamedItem := findItemByName(items, renamedFileName)
	if renamedItem == nil {
		t.Fatalf("Renamed file not found in folder listing")
	}

	t.Log("Step 5: Create test directory")
	testDirName := fmt.Sprintf("test_dir_%d", time.Now().Unix())
	if err := c.CreateFolderEncrypted(0, testDirName); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	t.Logf("✓ Created directory: %s", testDirName)

	time.Sleep(1 * time.Second)
	items, err = c.ListFolderEncrypted(0)
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
	dirItems, err := c.ListFolderEncrypted(testDirID)
	if err != nil {
		t.Fatalf("Failed to list test directory: %v", err)
	}
	movedItem := findItemByName(dirItems, renamedFileName)
	if movedItem == nil {
		t.Fatalf("File not found in test directory after move")
	}
	t.Logf("✓ Verified file in test directory")

	t.Log("Step 7: Download file and verify integrity")
	downloadDir := t.TempDir()
	if err := c.DownloadFileEncrypted(*movedItem, downloadDir); err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}

	downloadedPath := filepath.Join(downloadDir, renamedFileName)
	verifyFileHash(t, downloadedPath, originalHash)
	t.Logf("✓ File downloaded and integrity verified")

	t.Log("Cleanup: Deleting test directory")
	if err := c.Delete(*testDir); err != nil {
		t.Logf("Warning: Failed to delete test directory: %v", err)
	} else {
		t.Logf("✓ Cleaned up test directory")
	}

	t.Log("✅ Encrypted file upload workflow test completed successfully")
}
