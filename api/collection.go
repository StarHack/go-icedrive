package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type CollectionType string

const (
	CollectionCloud  CollectionType = "cloud"
	CollectionCrypto CollectionType = "crypto"
	CollectionTrash  CollectionType = "trash"
)

type Item struct {
	ID        uint64      `json:"id"`
	UID       string      `json:"uid"`
	Filename  string      `json:"filename"`
	ParentID  uint64      `json:"parentId"`
	Moddate   uint64      `json:"moddate"`
	IsFolder  int         `json:"isFolder"`
	Filesize  uint64      `json:"filesize"`
	Extension string      `json:"extension"`
	Fave      int         `json:"fave"`
	IsPublic  int         `json:"isPublic"`
	Color     interface{} `json:"color"`
	IsOwner   int         `json:"isOwner"`
	IsShared  int         `json:"isShared"`
	FileType  string      `json:"fileType"`
	Crypto    int         `json:"crypto"`
	Thumbnail interface{} `json:"thumbnail"`
}

type CollectionResponse struct {
	Error   bool   `json:"error"`
	Code    int    `json:"code,omitempty"`    // Error code (e.g., 1001 for auth error)
	Message string `json:"message,omitempty"` // Error message
	ID      uint64 `json:"id"`
	Access  string `json:"access"`
	Results int    `json:"results"`
	Data    []Item `json:"data"`
}

type FolderPropertiesResponse struct {
	Error      bool   `json:"error"`
	Code       int    `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	IsFolder   int    `json:"isFolder"`
	FolderID   uint64 `json:"folderId"`
	Filename   string `json:"filename"`
	Moddate    uint64 `json:"moddate"`
	NumFolders int    `json:"num_folders"`
	NumFiles   int    `json:"num_files"`
	TotalSize  uint64 `json:"total_size"`
	IsOwner    int    `json:"isOwner"`
	Owner      struct {
		Avatar string `json:"avatar"`
		Name   string `json:"name"`
	} `json:"owner"`
	Path string `json:"path"`
}

func GetCollection(h *HTTPClient, folderID uint64, cType CollectionType) (*CollectionResponse, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}

	if cType != CollectionCloud && cType != CollectionCrypto {
		return nil, fmt.Errorf("invalid collection type: %s", cType)
	}

	u := &url.URL{
		Path: "/collection",
	}
	q := url.Values{}
	q.Set("type", string(cType))
	q.Set("folderId", strconv.FormatUint(folderID, 10))
	u.RawQuery = q.Encode()

	status, _, body, err := h.httpGET(u.String())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("collection request failed with status %d", status)
	}

	var resp CollectionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		if resp.Code != 0 {
			return nil, fmt.Errorf("collection error (code %d): %s", resp.Code, resp.Message)
		}
		return nil, fmt.Errorf("collection error: %s", resp.Message)
	}

	if cType == CollectionCrypto {
		for i := range resp.Data {
			if decryptedFilename, err := DecryptFilename(h.GetCryptoKeyHex(), resp.Data[i].Filename); err == nil {
				resp.Data[i].Filename = decryptedFilename
			}
		}
	}

	return &resp, nil
}

func GetFolderProperties(h *HTTPClient, folderUID string, crypto bool) (*FolderPropertiesResponse, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}

	u := &url.URL{
		Path: "/folder-properties",
	}
	q := url.Values{}
	q.Set("id", folderUID)
	if crypto {
		q.Set("crypto", "1")
	} else {
		q.Set("crypto", "0")
	}
	u.RawQuery = q.Encode()

	status, _, body, err := h.httpGET(u.String())
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("folder-properties request failed with status %d", status)
	}

	var resp FolderPropertiesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Error {
		if resp.Code != 0 {
			return nil, fmt.Errorf("folder-properties error (code %d): %s", resp.Code, resp.Message)
		}
		return nil, fmt.Errorf("folder-properties error: %s", resp.Message)
	}

	if crypto {
		if decryptedFilename, err := DecryptFilename(h.GetCryptoKeyHex(), resp.Filename); err == nil {
			resp.Filename = decryptedFilename
		}
	}

	return &resp, nil
}
