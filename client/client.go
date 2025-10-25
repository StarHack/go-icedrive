package client

import (
	"errors"
	"io"

	"github.com/StarHack/go-icedrive/api"
)

const clientPoolSize = 3

type Client struct {
	pool           chan *api.HTTPClient
	all            []*api.HTTPClient
	hmacKeyHex     string
	user           *api.User
	Token          string
	cryptoPassword string
	CryptoSalt     string
	CryptoHexKey   string
}

func NewClient() *Client {
	c := &Client{
		pool: make(chan *api.HTTPClient, clientPoolSize),
		all:  make([]*api.HTTPClient, 0, clientPoolSize),
	}
	for i := 0; i < clientPoolSize; i++ {
		h := api.NewHTTPClientWithEnv()
		h.SetApiBase("https://apis.icedrive.net/v3/mobile")
		h.SetHeaders("User-Agent: icedrive-ios/2.2.2")
		c.pool <- h
		c.all = append(c.all, h)
	}
	c.hmacKeyHex = "436f6e67726174756c6174696f6e7320494620796f7520676f742054484953206661722121203b2921203a29"
	c.SetDebug(false)
	return c
}

func (c *Client) acquire() *api.HTTPClient {
	return <-c.pool
}

func (c *Client) release(h *api.HTTPClient) {
	c.pool <- h
}

func (c *Client) defaultAuthChecks(crypto bool) error {
	if c.user == nil {
		return errors.New("login first")
	}
	if crypto && c.CryptoHexKey == "" {
		return errors.New("set crypto password first")
	}
	return nil
}

func (c *Client) SetDebug(debug bool) {
	for _, h := range c.all {
		h.SetDebug(debug)
	}
}

func (c *Client) SetCryptoPassword(cryptoPassword string) {
	c.cryptoPassword = cryptoPassword
	if c.CryptoSalt == "" {
		h := c.acquire()
		_, salt, _ := api.FetchCryptoSaltAndStoredHash(h)
		c.release(h)
		c.CryptoSalt = salt
	}
	c.CryptoHexKey, _ = api.DeriveCryptoKey(cryptoPassword, c.CryptoSalt)
	for _, h := range c.all {
		h.SetCryptoKeyHex(c.CryptoHexKey)
	}
}

func (c *Client) LoginWithUsernameAndPassword(email, password string) error {
	h := c.acquire()
	user, err := api.LoginWithUsernameAndPassword(h, email, password, c.hmacKeyHex)
	if err != nil {
		c.release(h)
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
	c.release(h)
	for _, oh := range c.all {
		if oh.GetBearerToken() == c.Token {
			continue
		}
		_, _ = api.LoginWithBearerToken(oh, c.Token)
	}
	return nil
}

func (c *Client) LoginWithBearerToken(token string) error {
	h := c.acquire()
	user, err := api.LoginWithBearerToken(h, token)
	if err != nil {
		c.release(h)
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
	c.release(h)
	for _, oh := range c.all {
		if oh.GetBearerToken() == c.Token {
			continue
		}
		_, _ = api.LoginWithBearerToken(oh, c.Token)
	}
	return nil
}

func (c *Client) ListFolder(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.acquire()
	response, err := api.GetCollection(h, folderID, api.CollectionCloud)
	c.release(h)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderEncrypted(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.acquire()
	response, err := api.GetCollection(h, folderID, api.CollectionCrypto)
	c.release(h)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderTrash(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.acquire()
	response, err := api.GetCollection(h, folderID, api.CollectionTrash)
	c.release(h)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListVersions(item api.Item) ([]api.FileVersion, error) {
	h := c.acquire()
	v, err := api.ListVersions(h, item)
	c.release(h)
	return v, err
}

func (c *Client) CreateFolder(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.acquire()
	err := api.CreateFolder(h, parentID, name, false)
	c.release(h)
	return err
}

func (c *Client) CreateFolderEncrypted(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.acquire()
	err := api.CreateFolder(h, parentID, name, true)
	c.release(h)
	return err
}

func (c *Client) UploadFile(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.acquire()
	_, err := api.UploadFile(h, folderID, fileName)
	c.release(h)
	return err
}

func (c *Client) UploadFileEncrypted(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.acquire()
	_, err := api.UploadEncryptedFile(h, folderID, fileName, c.CryptoHexKey)
	c.release(h)
	return err
}

type wcWithRelease struct {
	io.WriteCloser
	release func()
}

func (w *wcWithRelease) Close() error {
	err := w.WriteCloser.Close()
	w.release()
	return err
}

func (c *Client) UploadFileWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.acquire()
	w, err := api.NewUploadFileWriter(h, folderID, fileName)
	if err != nil {
		c.release(h)
		return nil, err
	}
	return &wcWithRelease{WriteCloser: w, release: func() { c.release(h) }}, nil
}

func (c *Client) UploadFileEncryptedWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.acquire()
	w, err := api.NewUploadFileEncryptedWriter(h, folderID, fileName, c.CryptoHexKey)
	if err != nil {
		c.release(h)
		return nil, err
	}
	return &wcWithRelease{WriteCloser: w, release: func() { c.release(h) }}, nil
}

func (c *Client) DownloadFile(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.acquire()
	err := api.DownloadFile(h, item, destPath, false)
	c.release(h)
	return err
}

func (c *Client) DownloadFileStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.acquire()
	rc, err := api.OpenDownloadStream(h, item, false)
	if err != nil {
		c.release(h)
		return nil, err
	}
	return &rcWithRelease{ReadCloser: rc, release: func() { c.release(h) }}, nil
}

func (c *Client) DownloadFileEncrypted(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.acquire()
	err := api.DownloadFile(h, item, destPath, true)
	c.release(h)
	return err
}

type rcWithRelease struct {
	io.ReadCloser
	release func()
}

func (r *rcWithRelease) Close() error {
	err := r.ReadCloser.Close()
	r.release()
	return err
}

func (c *Client) DownloadFileEncryptedStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.acquire()
	rc, err := api.OpenDownloadStream(h, item, true)
	if err != nil {
		c.release(h)
		return nil, err
	}
	return &rcWithRelease{ReadCloser: rc, release: func() { c.release(h) }}, nil
}

func (c *Client) TrashItem(item api.Item) error {
	h := c.acquire()
	err := api.TrashAdd(h, item)
	c.release(h)
	return err
}

func (c *Client) TrashEraseAll() error {
	h := c.acquire()
	err := api.TrashEraseAll(h)
	c.release(h)
	return err
}

func (c *Client) RestoreTrashedItem(item api.Item) error {
	h := c.acquire()
	err := api.TrashRestore(h, item)
	c.release(h)
	return err
}

func (c *Client) Delete(item api.Item) error {
	h := c.acquire()
	err := api.Delete(h, item)
	c.release(h)
	return err
}

func (c *Client) Rename(item api.Item, newName string) error {
	h := c.acquire()
	var err error
	if item.IsFolder == 1 {
		err = api.RenameFolder(h, item, newName)
	} else {
		err = api.RenameFile(h, item, newName, false)
	}
	c.release(h)
	return err
}

func (c *Client) Move(targetFolderID uint64, items ...api.Item) error {
	h := c.acquire()
	err := api.Move(h, targetFolderID, items...)
	c.release(h)
	return err
}

func (c *Client) GetPlainSize(item api.Item) (int64, error) {
	if err := c.defaultAuthChecks(item.Crypto == 1); err != nil {
		return 0, err
	}
	h := c.acquire()
	sz, err := api.GetPlainSize(h, item, item.Crypto == 1)
	c.release(h)
	return sz, err
}
