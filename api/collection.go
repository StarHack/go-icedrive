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
)

type CollectionItem struct {
	ID        int64       `json:"id"`
	UID       string      `json:"uid"`
	Filename  string      `json:"filename"`
	ParentID  int64       `json:"parentId"`
	Moddate   int64       `json:"moddate"`
	IsFolder  int         `json:"isFolder"`
	Filesize  int64       `json:"filesize"`
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
	Error   bool             `json:"error"`
	ID      int64            `json:"id"`
	Access  string           `json:"access"`
	Results int              `json:"results"`
	Data    []CollectionItem `json:"data"`
}

func GetCollection(h *HTTPClient, folderID int64, cType CollectionType) (*CollectionResponse, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	if strings.TrimSpace(h.bearer) == "" {
		return nil, fmt.Errorf("missing bearer token; call Login first")
	}

	if cType != CollectionCloud && cType != CollectionCrypto {
		return nil, fmt.Errorf("invalid collection type: %s", cType)
	}

	u, _ := url.Parse("https://apis.icedrive.net/v3/webapp/collection")
	q := u.Query()
	q.Set("type", string(cType))
	q.Set("folderId", strconv.FormatInt(folderID, 10))
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
