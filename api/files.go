package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
)

func RenameFile(h *HTTPClient, item Item, newName string, keepExt bool) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "file-rename")
	_ = w.WriteField("filename", newName)
	_ = w.WriteField("id", item.UID)
	if keepExt {
		_ = w.WriteField("keep_ext", "true")
	} else {
		_ = w.WriteField("keep_ext", "false")
	}
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/file-rename", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("file-rename failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("file-rename error: %s", resp.Message)
	}
	return nil
}

func RenameFolder(h *HTTPClient, item Item, newName string) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "folder-rename")
	_ = w.WriteField("filename", newName)
	_ = w.WriteField("id", item.UID)
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/folder-rename", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("folder-rename failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("folder-rename error: %s", resp.Message)
	}
	return nil
}

func CreateFolder(h *HTTPClient, parentId uint64, name string) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "folder-create")
	_ = w.WriteField("type", "folder-create")
	_ = w.WriteField("parentId", strconv.FormatUint(parentId, 10))
	_ = w.WriteField("filename", name)
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/folder-create", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("folder-create failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("folder-create error: %s", resp.Message)
	}
	return nil
}

func Move(h *HTTPClient, folderId uint64, items ...Item) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	itemUIDs := []string{}
	for _, it := range items {
		if u := strings.TrimSpace(it.UID); u != "" {
			itemUIDs = append(itemUIDs, u)
		}
	}
	if len(itemUIDs) == 0 {
		return fmt.Errorf("no items provided")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "move")
	_ = w.WriteField("items", strings.Join(itemUIDs, ","))
	_ = w.WriteField("folderId", strconv.FormatUint(folderId, 10))
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/move", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("move failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("move error: %s", resp.Message)
	}
	return nil
}

func Delete(h *HTTPClient, item Item) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	uid := item.UID
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "erase")
	_ = w.WriteField("items", uid)
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/erase", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("erase failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("erase error: %s", resp.Message)
	}
	return nil
}
