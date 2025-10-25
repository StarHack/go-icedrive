package client

import (
	"errors"
	"io"
	"sync"

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
	select {
	case c.pool <- h:
	default:
	}
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
	c.release(h)
	if err != nil {
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
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
	c.release(h)
	if err != nil {
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
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
	defer c.release(h)
	response, err := api.GetCollection(h, folderID, api.CollectionCloud)
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
	defer c.release(h)
	response, err := api.GetCollection(h, folderID, api.CollectionCrypto)
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
	defer c.release(h)
	response, err := api.GetCollection(h, folderID, api.CollectionTrash)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListVersions(item api.Item) ([]api.FileVersion, error) {
	h := c.acquire()
	defer c.release(h)
	return api.ListVersions(h, item)
}

func (c *Client) CreateFolder(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.acquire()
	defer c.release(h)
	return api.CreateFolder(h, parentID, name, false)
}

func (c *Client) CreateFolderEncrypted(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.acquire()
	defer c.release(h)
	return api.CreateFolder(h, parentID, name, true)
}

func (c *Client) UploadFile(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.acquire()
	defer c.release(h)
	_, err := api.UploadFile(h, folderID, fileName)
	return err
}

func (c *Client) UploadFileEncrypted(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.acquire()
	defer c.release(h)
	_, err := api.UploadEncryptedFile(h, folderID, fileName, c.CryptoHexKey)
	return err
}

type wcWithRelease struct {
	io.WriteCloser
	once    sync.Once
	release func()
}

func (w *wcWithRelease) Close() error {
	var err error
	if w.WriteCloser != nil {
		err = w.WriteCloser.Close()
	}
	w.once.Do(w.release)
	return err
}

type rcWithRelease struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (r *rcWithRelease) Close() error {
	var err error
	if r.ReadCloser != nil {
		err = r.ReadCloser.Close()
	}
	r.once.Do(r.release)
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
	defer c.release(h)
	return api.DownloadFile(h, item, destPath, false)
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
	defer c.release(h)
	return api.DownloadFile(h, item, destPath, true)
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
	defer c.release(h)
	return api.TrashAdd(h, item)
}

func (c *Client) TrashEraseAll() error {
	h := c.acquire()
	defer c.release(h)
	return api.TrashEraseAll(h)
}

func (c *Client) RestoreTrashedItem(item api.Item) error {
	h := c.acquire()
	defer c.release(h)
	return api.TrashRestore(h, item)
}

func (c *Client) Delete(item api.Item) error {
	h := c.acquire()
	defer c.release(h)
	return api.Delete(h, item)
}

func (c *Client) Rename(item api.Item, newName string) error {
	h := c.acquire()
	defer c.release(h)
	if item.IsFolder == 1 {
		return api.RenameFolder(h, item, newName)
	}
	return api.RenameFile(h, item, newName, false)
}

func (c *Client) Move(targetFolderID uint64, items ...api.Item) error {
	h := c.acquire()
	defer c.release(h)
	return api.Move(h, targetFolderID, items...)
}

func (c *Client) GetPlainSize(item api.Item) (int64, error) {
	if err := c.defaultAuthChecks(item.Crypto == 1); err != nil {
		return 0, err
	}
	h := c.acquire()
	defer c.release(h)
	return api.GetPlainSize(h, item, item.Crypto == 1)
}
