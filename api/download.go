package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type DownloadURLEntry struct {
	ID       uint64 `json:"id"`
	Filename string `json:"filename"`
	Filesize uint64 `json:"filesize"`
	FolderID uint64 `json:"folderId"`
	Moddate  uint64 `json:"moddate"`
	Path     string `json:"path"`
	URL      string `json:"url"`
}

type DownloadMultiResponse struct {
	Error bool               `json:"error"`
	Urls  []DownloadURLEntry `json:"urls"`
}

func GetDownloadURLs(h *HTTPClient, itemUIDs []string, crypto bool) ([]DownloadURLEntry, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}
	u, _ := url.Parse("https://apis.icedrive.net/v3/webapp/download-multi")
	q := u.Query()
	q.Set("items", strings.Join(itemUIDs, ","))
	if crypto {
		q.Set("crypto", "true")
	} else {
		q.Set("crypto", "false")
	}
	u.RawQuery = q.Encode()

	status, _, body, err := h.httpGET(u.String())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("download-multi failed with status %d", status)
	}

	var resp DownloadMultiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		return nil, fmt.Errorf("download-multi error")
	}
	if len(resp.Urls) == 0 {
		return nil, fmt.Errorf("download-multi returned no urls")
	}
	return resp.Urls, nil
}

func DownloadFile(h *HTTPClient, item Item, destPath string, crypted bool) error {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	itemUID := item.UID
	fmt.Println("uga uga: " + itemUID)
	urls, err := GetDownloadURLs(h, []string{itemUID}, crypted)
	if err != nil {
		return err
	}
	dl := urls[0]
	dlURL := dl.URL

	destFilePath := filepath.Join(destPath, item.Filename)
	tmp := destFilePath + ".part"
	fmt.Println(tmp)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
	}()

	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return err
	}
	h.addEnvHeaders(req)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")

	res, err := h.c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("download GET failed: %s\n%s", res.Status, string(b))
	}

	if !crypted {
		buf := make([]byte, 2<<20)
		if _, err := io.CopyBuffer(out, res.Body, buf); err != nil {
			return err
		}
	} else {
		if err := DecryptTwofishCBCStream(out, res.Body, EnvCryptoKey64()); err != nil {
			return err
		}
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, destFilePath)
}
