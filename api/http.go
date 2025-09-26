package api

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

type HTTPClient struct {
	c   *http.Client
	jar http.CookieJar
}

func NewHTTPClientWithEnv() *HTTPClient {
	jar, _ := cookiejar.New(nil)
	return &HTTPClient{
		c: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		jar: jar,
	}
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

func addEnvHeaders(req *http.Request) {
	for _, kv := range parseEnvHeaders(EnvAPIHeaders()) {
		req.Header.Set(kv[0], kv[1])
	}
	if ck := EnvCookie(); ck != "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", ck)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0")
	}
}

func printHeaders(req *http.Request) {
	fmt.Println(">>> HTTP Request:", req.Method, req.URL.String())
	for k, v := range req.Header {
		fmt.Printf("%s: %s\n", k, strings.Join(v, "; "))
	}
	fmt.Println(">>> End Headers")
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

func (h *HTTPClient) httpGET(u string) (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}
	req, _ := http.NewRequest("GET", u, nil)
	addEnvHeaders(req)
	printHeaders(req)
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

func (h *HTTPClient) httpPOST(u string, contentType string, body []byte) (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}
	req, _ := http.NewRequest("POST", u, bytes.NewReader(body))
	addEnvHeaders(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	printHeaders(req)
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

func (h *HTTPClient) InjectEnvCookies() {
	ck := EnvCookie()
	if ck == "" || h.jar == nil {
		return
	}
	parseCookieStr := func(raw string) []*http.Cookie {
		var out []*http.Cookie
		parts := strings.Split(raw, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				continue
			}
			out = append(out, &http.Cookie{
				Name:  strings.TrimSpace(kv[0]),
				Value: strings.TrimSpace(kv[1]),
				Path:  "/",
			})
		}
		return out
	}
	for _, host := range []string{"https://icedrive.net", "https://apis.icedrive.net"} {
		u, _ := url.Parse(host)
		h.jar.SetCookies(u, parseCookieStr(ck))
	}
}

func (h *HTTPClient) Preflight() (int, http.Header, []byte, error) {
	if h == nil || h.c == nil {
		h = NewHTTPClientWithEnv()
	}
	h.InjectEnvCookies()
	s1, _, _, err := h.httpGET("https://icedrive.net/")
	if err != nil || s1 < 200 || s1 >= 400 {
		return s1, nil, nil, err
	}
	return h.httpGET("https://icedrive.net/login")
}
