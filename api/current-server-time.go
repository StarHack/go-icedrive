package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// ServerTimeResponse matches the JSON structure returned by Icedrive.
type ServerTimeResponse struct {
	Error    bool   `json:"error"`
	Message  string `json:"message"`
	TimeUnix uint64 `json:"time_unix"`
}

// GetServerTime fetches the current server time from Icedrive API.
func GetServerTime(client *http.Client) (*ServerTimeResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	url := "https://apis.icedrive.net/v3/website/current-server-time"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp ServerTimeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
