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
	ID      uint64 `json:"id"`
	Access  string `json:"access"`
	Results int    `json:"results"`
	Data    []Item `json:"data"`
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
		if h.debug {
			fmt.Println(string(body))
		}
		return nil, fmt.Errorf("collection error")
	}

	if cType == CollectionCrypto {
		for i := range resp.Data {
			if decryptedFilename, err := DecryptFilename(EnvCryptoKey64(), resp.Data[i].Filename); err == nil {
				resp.Data[i].Filename = decryptedFilename
			}
		}
	}

	return &resp, nil
}
