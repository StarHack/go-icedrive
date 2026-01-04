package api

import (
	"bytes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/twofish"
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

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("items", strings.Join(itemUIDs, ","))
	if crypto {
		w.WriteField("crypto", "1")
	}
	w.Close()

	status, _, body, err := h.httpPOSTReader("/download-multi", w.FormDataContentType(), &b)
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
	h.addHeaders(req)
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
		if err := DecryptTwofishCBCStream(out, res.Body, h.GetCryptoKeyHex()); err != nil {
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

func OpenDownloadStream(h *HTTPClient, item Item, crypted bool) (io.ReadCloser, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}
	urls, err := GetDownloadURLs(h, []string{item.UID}, crypted)
	if err != nil {
		return nil, err
	}
	dl := urls[0]
	req, err := http.NewRequest("GET", dl.URL, nil)
	if err != nil {
		return nil, err
	}
	h.addHeaders(req)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := h.c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download GET failed: %s\n%s", resp.Status, string(b))
	}
	if !crypted {
		return resp.Body, nil
	}
	pr, pw := io.Pipe()
	go func() {
		defer resp.Body.Close()
		if err := DecryptTwofishCBCStream(pw, resp.Body, h.GetCryptoKeyHex()); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return struct {
		io.Reader
		io.Closer
	}{Reader: pr, Closer: resp.Body}, nil
}

func GetPlainSize(h *HTTPClient, item Item, crypted bool) (int64, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return 0, fmt.Errorf("missing bearer token; call Login first")
	}
	urls, err := GetDownloadURLs(h, []string{item.UID}, crypted)
	if err != nil {
		return 0, err
	}
	dl := urls[0]

	headReq, err := http.NewRequest("HEAD", dl.URL, nil)
	if err != nil {
		return 0, err
	}
	h.addHeaders(headReq)
	headReq.Header.Set("Accept", "*/*")
	resp, err := h.c.Do(headReq)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("HEAD failed: %s", resp.Status)
	}
	clStr := resp.Header.Get("Content-Length")
	if clStr == "" {
		return 0, fmt.Errorf("missing Content-Length")
	}
	var total int64
	_, err = fmt.Sscanf(clStr, "%d", &total)
	if err != nil {
		return 0, err
	}
	if !crypted {
		return total, nil
	}

	rReq, err := http.NewRequest("GET", dl.URL, nil)
	if err != nil {
		return 0, err
	}
	h.addHeaders(rReq)
	rReq.Header.Set("Accept", "*/*")
	rReq.Header.Set("Range", "bytes=0-31")
	rResp, err := h.c.Do(rReq)
	if err != nil {
		return 0, err
	}
	defer rResp.Body.Close()
	if rResp.StatusCode != http.StatusPartialContent && rResp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("range GET failed: %s", rResp.Status)
	}
	headerCipher, err := io.ReadAll(rResp.Body)
	if err != nil {
		return 0, err
	}
	if len(headerCipher) < 32 {
		return 0, fmt.Errorf("short header")
	}

	keyHex := h.GetCryptoKeyHex()
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return 0, err
	}
	block, err := twofish.NewCipher(key)
	if err != nil {
		return 0, err
	}
	iv := []byte("1234567887654321")
	cbc := cipher.NewCBCDecrypter(block, iv)
	headerPlain := make([]byte, 32)
	cbc.CryptBlocks(headerPlain, headerCipher[:32])

	numPadding := int(headerPlain[16])
	plain := (total - 32) - int64(numPadding)
	if plain < 0 {
		return 0, fmt.Errorf("calculated negative size")
	}
	return plain, nil
}
