package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type GeoFileserverList struct {
	Error           bool     `json:"error"`
	UploadEndpoints []string `json:"upload_endpoints"`
}

type UploadFileObj struct {
	ID          uint64      `json:"id"`
	UID         string      `json:"uid"`
	Type        string      `json:"type"`
	IsFolder    int         `json:"isFolder"`
	Filename    string      `json:"filename"`
	FilenameRaw string      `json:"filename_raw"`
	Filesize    uint64      `json:"filesize"`
	Filemod     uint64      `json:"filemod"`
	Moddate     uint64      `json:"moddate"`
	FileType    string      `json:"fileType"`
	Extension   string      `json:"extension"`
	ServerID    int         `json:"serverId"`
	IsPublic    int         `json:"isPublic"`
	Thumbnail   interface{} `json:"thumbnail"`
	Crypto      int         `json:"crypto"`
	Padding     int         `json:"padding"`
	Fave        int         `json:"fave"`
	FolderID    uint64      `json:"folderId"`
	Overwrite   bool        `json:"overwrite"`
}

type UploadResponse struct {
	Error     bool          `json:"error"`
	Message   string        `json:"message"`
	ID        uint64        `json:"id"`
	Time      uint64        `json:"time"`
	Overwrite bool          `json:"overwrite"`
	FolderID  uint64        `json:"folderId"`
	FileObj   UploadFileObj `json:"fileObj"`
}

func GetUploadEndpoints(h *HTTPClient) ([]string, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	status, _, body, err := h.httpGET("/geo-fileserver-list")
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("geo-fileserver-list failed with status %d", status)
	}
	var resp GeoFileserverList
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		return nil, fmt.Errorf("geo-fileserver-list error")
	}
	return resp.UploadEndpoints, nil
}

func UploadFile(h *HTTPClient, folderID uint64, filename string) (*UploadResponse, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	endpoints, err := GetUploadEndpoints(h)
	if err != nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("no upload endpoints: %w", err)
	}
	endpoint := endpoints[0]
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	moddate := float64(fi.ModTime().UnixNano()) / 1e9
	ext := strings.ToLower(filepath.Ext(filename))
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		head := make([]byte, 512)
		n, _ := f.Read(head)
		_, _ = f.Seek(0, 0)
		ct = http.DetectContentType(head[:n])
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	_ = w.SetBoundary("----geckoformboundary" + randHex(16))
	go func() {
		_ = w.WriteField("folderId", strconv.FormatUint(folderID, 10))
		_ = w.WriteField("moddate", strconv.FormatFloat(moddate, 'f', -1, 64))
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="files[]"; filename="`+filepath.Base(filename)+`"`)
		hdr.Set("Content-Type", ct)
		part, err := w.CreatePart(hdr)
		if err == nil {
			_, err = io.Copy(part, f)
		}
		_ = w.Close()
		_ = pw.CloseWithError(err)
	}()
	status, _, body, err := h.httpPOSTReader(endpoint, w.FormDataContentType(), pr)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("upload failed with status %d", status)
	}
	var out UploadResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error {
		return nil, fmt.Errorf("upload error: %s", out.Message)
	}
	return &out, nil
}

func UploadEncryptedFile(h *HTTPClient, folderID uint64, filename string, hexkey string) (*UploadResponse, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	endpoints, err := GetUploadEndpoints(h)
	if err != nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("no upload endpoints: %w", err)
	}
	endpoint := endpoints[0]
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	moddate := float64(fi.ModTime().UnixNano()) / 1e9
	ext := strings.ToLower(filepath.Ext(filename))
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		head := make([]byte, 512)
		n, _ := f.Read(head)
		_, _ = f.Seek(0, 0)
		ct = http.DetectContentType(head[:n])
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	_ = w.SetBoundary("----geckoformboundary" + randHex(16))
	go func() {
		encryptedFilename, err := EncryptFilename(hexkey, filename)

		_ = w.WriteField("folderId", strconv.FormatUint(folderID, 10))
		_ = w.WriteField("moddate", strconv.FormatFloat(moddate, 'f', -1, 64))
		_ = w.WriteField("custom_filename", encryptedFilename)
		_ = w.WriteField("crypto", "1")
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="files[]"; filename="`+filepath.Base(filename)+`"`)
		hdr.Set("Content-Type", ct)
		part, err := w.CreatePart(hdr)
		if err == nil {
			err = EncryptTwofishCBCStream(part, f, hexkey, uint64(fi.Size()))
		}
		_ = w.Close()
		_ = pw.CloseWithError(err)
	}()
	status, _, body, err := h.httpPOSTReader(endpoint, w.FormDataContentType(), pr)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("upload failed with status %d", status)
	}
	var out UploadResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error {
		return nil, fmt.Errorf("upload error: %s", out.Message)
	}
	return &out, nil
}

func NewUploadFileWriter(h *HTTPClient, folderID uint64, filename string) (io.WriteCloser, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	endpoints, err := GetUploadEndpoints(h)
	if err != nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("no upload endpoints: %w", err)
	}
	endpoint := endpoints[0]

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	moddate := float64(fi.ModTime().UnixNano()) / 1e9
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if ct == "" {
		ct = "application/octet-stream"
	}

	pr, pw := io.Pipe()
	mp := multipart.NewWriter(pw)
	_ = mp.SetBoundary("----geckoformboundary" + randHex(16))

	partR, partW := io.Pipe()

	go func() {
		_ = mp.WriteField("folderId", strconv.FormatUint(folderID, 10))
		_ = mp.WriteField("moddate", strconv.FormatFloat(moddate, 'f', -1, 64))
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="files[]"; filename="`+filepath.Base(filename)+`"`)
		hdr.Set("Content-Type", ct)
		part, err := mp.CreatePart(hdr)
		if err == nil {
			_, err = io.Copy(part, partR)
		}
		_ = mp.Close()
		_ = pw.CloseWithError(err)
	}()

	go func() {
		_, _, _, _ = h.httpPOSTReader(endpoint, mp.FormDataContentType(), pr)
	}()

	return partW, nil
}

func NewUploadFileEncryptedWriter(h *HTTPClient, folderID uint64, filename string, hexkey string) (io.WriteCloser, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	endpoints, err := GetUploadEndpoints(h)
	if err != nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("no upload endpoints: %w", err)
	}
	endpoint := endpoints[0]

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	moddate := float64(fi.ModTime().UnixNano()) / 1e9
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if ct == "" {
		ct = "application/octet-stream"
	}

	pr, pw := io.Pipe()
	mp := multipart.NewWriter(pw)
	_ = mp.SetBoundary("----geckoformboundary" + randHex(16))

	partR, partW := io.Pipe()

	go func() {
		encryptedFilename, _ := EncryptFilename(hexkey, filename)
		_ = mp.WriteField("folderId", strconv.FormatUint(folderID, 10))
		_ = mp.WriteField("moddate", strconv.FormatFloat(moddate, 'f', -1, 64))
		_ = mp.WriteField("custom_filename", encryptedFilename)
		_ = mp.WriteField("crypto", "1")
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="files[]"; filename="`+filepath.Base(filename)+`"`)
		hdr.Set("Content-Type", ct)
		part, err := mp.CreatePart(hdr)
		if err == nil {
			err = EncryptTwofishCBCStream(part, partR, hexkey, uint64(fi.Size()))
		}
		_ = mp.Close()
		_ = pw.CloseWithError(err)
	}()

	go func() {
		_, _, _, _ = h.httpPOSTReader(endpoint, mp.FormDataContentType(), pr)
	}()

	return partW, nil
}
