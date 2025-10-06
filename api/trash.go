package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strings"
)

type TrashAddResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

type TrashEraseAllResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

func TrashAdd(h *HTTPClient, items ...Item) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	itemUIDs := []string{}
	for _, item := range items {
		itemUIDs = append(itemUIDs, item.UID)
	}
	if len(itemUIDs) == 0 {
		return fmt.Errorf("no items provided")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.SetBoundary("----geckoformboundary" + randHex(16))
	_ = w.WriteField("request", "trash-add")
	_ = w.WriteField("items", strings.Join(itemUIDs, ","))
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/trash-add", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("trash-add failed with status %d", status)
	}
	var resp TrashAddResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("trash-add error: %s", resp.Message)
	}
	return nil
}

func TrashEraseAll(h *HTTPClient) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return fmt.Errorf("missing bearer token; call Login first")
	}
	status, _, body, err := h.httpGET("/trash-erase-all")
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("trash-erase-all failed with status %d", status)
	}
	var resp TrashEraseAllResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("trash-erase-all error: %s", resp.Message)
	}
	return nil
}

func TrashRestore(h *HTTPClient, item Item) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	uid := item.UID
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("request", "trash-restore")
	_ = w.WriteField("items", uid)
	if err := w.Close(); err != nil {
		return err
	}
	status, _, body, err := h.httpPOST("/trash-restore", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("trash-restore failed with status %d", status)
	}
	var resp struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error {
		return fmt.Errorf("trash-restore error: %s", resp.Message)
	}
	return nil
}
