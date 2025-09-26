package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
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

func LoginWithUsernameAndPassword(h *HTTPClient, email, password, hmacKeyHex string) (int, http.Header, []byte, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	sts, _, stBody, err := h.httpGET("https://apis.icedrive.net/v3/website/current-server-time")
	if err != nil || sts < 200 || sts >= 400 {
		return sts, nil, stBody, err
	}
	var st struct {
		TimeUnix int64 `json:"time_unix"`
	}
	if err := json.Unmarshal(stBody, &st); err != nil {
		return sts, nil, stBody, err
	}
	formSecure, err := ComputeProofOfWork(st.TimeUnix, hmacKeyHex)
	if err != nil {
		return 0, nil, nil, err
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
	code, hdr, body, err := h.httpPOST("https://apis.icedrive.net/v3/website/login", ct, buf.Bytes())
	if code >= 200 && code < 400 && err == nil {
		var lr LoginResponse
		if json.Unmarshal(body, &lr) == nil && lr.Token != "" {
			h.SetBearerToken(lr.Token)
		}
	}
	return code, hdr, body, err
}

func LoginWithBearerToken(h *HTTPClient, token string) (*User, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	h.SetBearerToken(token)
	return UserData(h)
}

