package api

import (
	"encoding/json"
	"fmt"
)

type User struct {
	ID          uint64 `json:"id"`
	Email       string `json:"email"`
	FullName    string `json:"fullName"`
	Plan        string `json:"plan"`
	LevelID     uint64 `json:"level_id"`
	LevelType   string `json:"level_type"`
	AvatarURL   string `json:"avatar_url"`
	APIKey      string `json:"apiKey"`
	BearerToken bool   `json:"bearer_token"`
	Error       bool   `json:"error"`
}

func UserData(h *HTTPClient) (*User, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}

	status, _, body, err := h.httpGET("/user-data")
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("user-data request failed with status %d", status)
	}

	var resp User
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
