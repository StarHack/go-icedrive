package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type FileVersion struct {
	Current   bool   `json:"current"`
	Date      string `json:"date"`
	Timestamp int64  `json:"timestamp"`
	Filesize  int64  `json:"filesize"`
	URL       string `json:"url"`
}

type VersionListResponse struct {
	Error    bool          `json:"error"`
	Filename string        `json:"filename"`
	Versions []FileVersion `json:"versions"`
}

func ListVersions(h *HTTPClient, item Item) ([]FileVersion, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if item.UID == "" {
		return nil, fmt.Errorf("missing item UID")
	}
	u, _ := url.Parse("https://apis.icedrive.net/v3/webapp/version-list")
	q := u.Query()
	q.Set("id", item.UID)
	u.RawQuery = q.Encode()
	status, _, body, err := h.httpGET(u.String())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("version-list failed with status %d", status)
	}
	var resp VersionListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		return nil, fmt.Errorf("version-list error for %s", item.UID)
	}
	return resp.Versions, nil
}
