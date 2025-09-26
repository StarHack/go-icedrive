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

func TrashAdd(h *HTTPClient, itemUIDs ...string) (string, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if len(itemUIDs) == 0 {
		return "", fmt.Errorf("no items provided")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.SetBoundary("----geckoformboundary" + randHex(16))
	_ = w.WriteField("request", "trash-add")
	_ = w.WriteField("items", strings.Join(itemUIDs, ","))
	if err := w.Close(); err != nil {
		return "", err
	}
	status, _, body, err := h.httpPOST("https://apis.icedrive.net/v3/webapp/trash-add", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("trash-add failed with status %d", status)
	}
	var resp TrashAddResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Error {
		return "", fmt.Errorf("trash-add error: %s", resp.Message)
	}
	return resp.Message, nil
}

func TrashEraseAll(h *HTTPClient) (string, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return "", fmt.Errorf("missing bearer token; call Login first")
	}
	status, _, body, err := h.httpGET("https://apis.icedrive.net/v3/webapp/trash-erase-all")
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("trash-erase-all failed with status %d", status)
	}
	var resp TrashEraseAllResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Error {
		return "", fmt.Errorf("trash-erase-all error: %s", resp.Message)
	}
	return resp.Message, nil
}