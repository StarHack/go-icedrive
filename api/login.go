package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
)

type LoginAuthData struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	LevelID     string      `json:"level_id"`
	APIKey      string      `json:"apiKey"`
	FullName    string      `json:"fullName"`
	LevelType   string      `json:"level_type"`
	Plan        string      `json:"plan"`
	AvatarURL   string      `json:"avatar_url"`
	Token       interface{} `json:"token"`
	BearerToken bool        `json:"bearer_token"`
}

type LoginResponse struct {
	Error    bool          `json:"error"`
	Message  string        `json:"message"`
	Token    string        `json:"token"`
	AuthData LoginAuthData `json:"auth_data"`
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func LoginWithUsernameAndPassword(h *HTTPClient, email, password, hmacKeyHex string) (*User, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	// Fetch new POW challenge
	challenge, err := FetchPOWChallenge(h, "login")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch POW challenge: %w", err)
	}

	// Solve the challenge (returns base64-encoded nonce and hex hash)
	nonceB64, hash, err := SolvePOWChallenge(challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to solve POW challenge: %w", err)
	}

	// Prepare the POW proof structure
	powProof := map[string]interface{}{
		"client_id":      "",
		"token":          challenge.Token,
		"challenge":      challenge.Challenge,
		"ver":            "1",
		"hash":           hash,
		"nonce":          nonceB64,
		"exp":            challenge.Exp,
		"difficultyBits": challenge.DifficultyBits,
		"scope":          challenge.Scope,
	}
	powProofBytes, err := json.Marshal(powProof)
	if err != nil {
		return nil, err
	}
	powProofStr := base64.StdEncoding.EncodeToString(powProofBytes)

	// Prepare URL-encoded form data
	formData := url.Values{}
	formData.Set("password", password)
	formData.Set("pow_proof", powProofStr)
	formData.Set("request", "login")
	formData.Set("email", email)
	formData.Set("no_token_check", "true")
	formData.Set("app", "ios")
	payload := formData.Encode()

	code, _, body, err := h.httpPOST("/api", "application/x-www-form-urlencoded", []byte(payload))
	if err != nil {
		return nil, err
	}
	if code < 200 || code >= 400 {
		return nil, fmt.Errorf("login failed: HTTP %d, body: %s", code, string(body))
	}
	var lr LoginResponse
	if err = json.Unmarshal(body, &lr); err != nil {
		return nil, err
	}
	if lr.Token == "" {
		return nil, fmt.Errorf("login response missing token")
	}
	userData, err := UserData(h)
	if err != nil {
		return nil, err
	}
	h.SetBearerToken(lr.Token)
	return userData, nil
}

func LoginWithBearerToken(h *HTTPClient, token string) (*User, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	h.SetBearerToken(token)
	userData, err := UserData(h)
	if err != nil {
		return nil, err
	}
	return userData, nil
}
