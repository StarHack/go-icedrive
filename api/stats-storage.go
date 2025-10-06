package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

type StorageStats struct {
	Error     bool    `json:"error"`
	Used      uint64  `json:"used"`
	UsedHuman string  `json:"used_human"`
	Max       uint64  `json:"max"`
	MaxHuman  string  `json:"max_human"`
	Free      uint64  `json:"free"`
	FreeHuman string  `json:"free_human"`
	Pcent     int     `json:"pcent"`
	PcentRaw  float64 `json:"pcent_raw"`
}

func GetStorageStats(h *HTTPClient) (*StorageStats, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}
	status, _, body, err := h.httpGET("/stats-storage")
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("stats-storage request failed with status %d", status)
	}
	var resp StorageStats
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		return nil, fmt.Errorf("stats-storage error")
	}
	return &resp, nil
}
