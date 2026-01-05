package api

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

// APIError represents a standard error response from the API
type APIError struct {
	Error   bool   `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IsAuthError checks if the error is an authentication error (code 1001)
func (e *APIError) IsAuthError() bool {
	return e.Error && e.Code == 1001
}

// ReloginFunc is a function that can re-authenticate the client
type ReloginFunc func() error

type HTTPClient struct {
	c            *http.Client
	jar          http.CookieJar
	bearer       string
	debug        bool
	headers      string
	apiBase      string
	cryptoKeyHex string

	// For automatic re-login
	reloginFunc  ReloginFunc
	reloginMutex sync.Mutex
}

func NewHTTPClientWithEnv() *HTTPClient {
	jar, _ := cookiejar.New(nil)
	h := &HTTPClient{
		c: &http.Client{
			Timeout: 600 * time.Second,
			Jar:     jar,
		},
		jar: jar,
	}
	return h
}

func (h *HTTPClient) SetBearerToken(t string) {
	h.bearer = t
}

func (h *HTTPClient) SetCryptoKeyHex(hex string) {
	h.cryptoKeyHex = hex
}

func (h *HTTPClient) GetBearerToken() string {
	return h.bearer
}

func (h *HTTPClient) GetCryptoKeyHex() string {
	return h.cryptoKeyHex
}

var headerWhitelist = []string{
	"User-Agent",
	"Accept",
	"Accept-Language",
	"Accept-Encoding",
	"Referer",
	"Origin",
	"Connection",
	"Upgrade-Insecure-Requests",
	"Sec-Fetch-Dest",
	"Sec-Fetch-Mode",
	"Sec-Fetch-Site",
	"Sec-Fetch-User",
	"Sec-GPC",
	"Priority",
	"TE",
	"Content-Type",
	"Authorization",
}

func parseEnvHeaders(hstr string) [][2]string {
	h := strings.TrimSpace(hstr)
	if h == "" {
		return nil
	}
	lh := strings.ToLower(h)
	lnames := make([]string, len(headerWhitelist))
	for i, n := range headerWhitelist {
		lnames[i] = strings.ToLower(n)
	}
	nextHeaderAt := func(from int) (idx int, name string) {
		min := -1
		var found string
		for i, ln := range lnames {
			p := strings.Index(lh[from:], ln+":")
			if p >= 0 {
				pos := from + p
				if min == -1 || pos < min {
					min = pos
					found = headerWhitelist[i]
				}
			}
		}
		return min, found
	}
	var out [][2]string
	i := 0
	for {
		idx, name := nextHeaderAt(i)
		if idx == -1 {
			break
		}
		colon := idx + len(name)
		for colon < len(h) && h[colon] != ':' {
			colon++
		}
		if colon >= len(h) || h[colon] != ':' {
			i = idx + 1
			continue
		}
		valStart := colon + 1
		for valStart < len(h) && (h[valStart] == ' ' || h[valStart] == '\t') {
			valStart++
		}
		nxtIdx, _ := nextHeaderAt(valStart)
		valEnd := len(h)
		if nxtIdx != -1 {
			valEnd = nxtIdx
		}
		val := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(h[valStart:valEnd]), ";"))
		out = append(out, [2]string{name, val})
		i = valEnd
	}
	return out
}

func (h *HTTPClient) SetApiBase(apiBase string) {
	h.apiBase = apiBase
}

func (h *HTTPClient) SetHeaders(headers string) {
	h.headers = headers
}

func (h *HTTPClient) addHeaders(req *http.Request) {
	for _, kv := range parseEnvHeaders(h.headers) {
		if strings.EqualFold(kv[0], "authorization") && h.bearer != "" {
			continue
		}
		req.Header.Set(kv[0], kv[1])
	}
	if h.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+h.bearer)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	}
	if req.Header.Get("Origin") == "" {
		//req.Header.Set("Origin", "https://icedrive.net")
	}
	if req.Header.Get("Referer") == "" {
		//req.Header.Set("Referer", "https://icedrive.net/")
	}
}

func (h *HTTPClient) printHeaders(req *http.Request) {
	// Debug logging disabled
}

func decodeBody(res *http.Response) ([]byte, error) {
	var r io.ReadCloser
	switch strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Encoding"))) {
	case "gzip":
		zr, err := gzip.NewReader(res.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	case "deflate":
		zr, err := zlib.NewReader(res.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	case "br":
		r = io.NopCloser(brotli.NewReader(res.Body))
	default:
		r = res.Body
	}
	return io.ReadAll(r)
}

// withRetryOnAuthError wraps an HTTP operation and retries it after re-login if auth fails
func (h *HTTPClient) withRetryOnAuthError(operation func() (int, http.Header, []byte, error)) (int, http.Header, []byte, error) {
	status, headers, body, err := operation()

	// Check for HTTP-level auth errors (401 Unauthorized, 403 Forbidden)
	if err == nil && (status == 401 || status == 403) {
		h.reloginMutex.Lock()
		reloginFunc := h.reloginFunc
		h.reloginMutex.Unlock()

		if reloginFunc != nil {
			// Clear the invalid token before attempting re-login
			oldToken := h.bearer
			h.bearer = ""

			reloginErr := reloginFunc()

			if reloginErr == nil {
				// Re-login succeeded, retry the original operation
				return operation()
			} else {
				// Re-login failed, restore the old token (even though it's invalid)
				h.bearer = oldToken
			}
		}
	}

	// If the request succeeded at HTTP level, check for API-level auth errors
	if err == nil && body != nil {
		apiErr, parseErr := tryParseAPIError(body)

		if parseErr == nil && apiErr != nil && apiErr.IsAuthError() {
			// Authentication error detected - try to re-login once
			h.reloginMutex.Lock()
			reloginFunc := h.reloginFunc
			h.reloginMutex.Unlock()

			if reloginFunc != nil {
				// Clear the invalid token before attempting re-login
				oldToken := h.bearer
				h.bearer = ""

				reloginErr := reloginFunc()

				if reloginErr == nil {
					// Re-login succeeded, retry the original operation
					return operation()
				} else {
					// Re-login failed, restore the old token (even though it's invalid)
					h.bearer = oldToken
				}
			}
		}
	}

	return status, headers, body, err
}

func (h *HTTPClient) httpGET(u string) (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}

	operation := func() (int, http.Header, []byte, error) {
		req, _ := http.NewRequest("GET", h.apiBase+u, nil)
		h.addHeaders(req)
		h.printHeaders(req)
		res, err := h.c.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer res.Body.Close()
		b, err := decodeBody(res)
		if err != nil {
			return res.StatusCode, res.Header, nil, err
		}
		// Debug logging disabled
		return res.StatusCode, res.Header, b, nil
	}

	return h.withRetryOnAuthError(operation)
}

func (h *HTTPClient) httpPOST(u string, contentType string, body []byte) (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}

	operation := func() (int, http.Header, []byte, error) {
		req, _ := http.NewRequest("POST", h.apiBase+u, bytes.NewReader(body))
		h.addHeaders(req)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		h.printHeaders(req)
		res, err := h.c.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer res.Body.Close()
		b, err := decodeBody(res)
		if err != nil {
			return res.StatusCode, res.Header, nil, err
		}
		return res.StatusCode, res.Header, b, nil
	}

	return h.withRetryOnAuthError(operation)
}

func (h *HTTPClient) httpPOSTReader(u string, contentType string, body io.Reader) (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}

	operation := func() (int, http.Header, []byte, error) {
		url := u
		if strings.HasPrefix(url, "/") {
			url = h.apiBase + u
		}
		req, _ := http.NewRequest("POST", url, body)
		h.addHeaders(req)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		h.printHeaders(req)
		res, err := h.c.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer res.Body.Close()
		b, err := decodeBody(res)
		if err != nil {
			return res.StatusCode, res.Header, nil, err
		}
		return res.StatusCode, res.Header, b, nil
	}

	return h.withRetryOnAuthError(operation)
}

func (h *HTTPClient) SetDebug(debug bool) {
	h.debug = debug
}

// SetReloginFunc sets the function to call when authentication fails
func (h *HTTPClient) SetReloginFunc(fn ReloginFunc) {
	h.reloginMutex.Lock()
	defer h.reloginMutex.Unlock()
	h.reloginFunc = fn
}

// tryParseAPIError attempts to parse an API error from the response body
func tryParseAPIError(body []byte) (*APIError, error) {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return nil, err
	}
	if apiErr.Error {
		return &apiErr, nil
	}
	return nil, nil
}
