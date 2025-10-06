package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
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
	sts, _, stBody, err := h.httpGET("/current-server-time")
	if err != nil || sts < 200 || sts >= 400 {
		return nil, err
	}
	var st struct {
		TimeUnix uint64 `json:"time_unix"`
	}
	if err := json.Unmarshal(stBody, &st); err != nil {
		return nil, err
	}
	formSecure, err := ComputeProofOfWork(st.TimeUnix, hmacKeyHex)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.SetBoundary("----geckoformboundary" + randHex(16))
	_ = w.WriteField("e-mail", "")
	_ = w.WriteField("email", email)
	_ = w.WriteField("password", password)
	_ = w.WriteField("form_secure", formSecure)
	_ = w.Close()
	ct := w.FormDataContentType()

	code, _, body, err := h.httpPOST("/login", ct, buf.Bytes())
	if code >= 200 && code < 400 && err == nil {
		if h.debug {
			fmt.Println(string(body))
		}
		var lr LoginResponse
		if err = json.Unmarshal(body, &lr); err == nil && lr.Token != "" {
			userData, err := UserData(h)
			if err != nil {
				return nil, err
			}
			h.SetBearerToken(lr.Token)
			return userData, nil
		} else {
			return nil, err
		}
	}
	return nil, err
}

func LoginWithBearerToken(h *HTTPClient, token string) (*User, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	userData, err := UserData(h)
	if err != nil {
		return nil, err
	}
	h.SetBearerToken(token)
	return userData, nil
}
