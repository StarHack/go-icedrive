package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Login(h *HTTPClient, email, password, hmacKeyHex string) (int, http.Header, []byte, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}

	stStatus, _, stBody, err := h.httpGET("https://apis.icedrive.net/v3/website/current-server-time")
	if err != nil {
		return stStatus, nil, stBody, err
	}
	var st struct {
		Error    bool   `json:"error"`
		Message  string `json:"message"`
		TimeUnix int64  `json:"time_unix"`
	}
	if err := json.Unmarshal(stBody, &st); err != nil {
		return stStatus, nil, stBody, err
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
	return h.httpPOST("https://apis.icedrive.net/v3/website/login", ct, buf.Bytes())
}
