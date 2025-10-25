package client

import (
	"errors"
	"io"
	"sync"

	"github.com/StarHack/go-icedrive/api"
)

type httpClientPool struct {
	ch        chan *api.HTTPClient
	all       []*api.HTTPClient
	mu        sync.Mutex
	base      string
	headers   []string
	debug     bool
	bearer    string
	cryptoHex string
}

func newHTTPClientPool(size int) *httpClientPool {
	p := &httpClientPool{
		ch: make(chan *api.HTTPClient, size),
	}
	for i := 0; i < size; i++ {
		h := api.NewHTTPClientWithEnv()
		p.all = append(p.all, h)
		p.ch <- h
	}
	return p
}

func (p *httpClientPool) configureClient(h *api.HTTPClient) {
	h.SetDebug(p.debug)
	if p.base != "" {
		h.SetApiBase(p.base)
	}
	for _, hdr := range p.headers {
		h.SetHeaders(hdr)
	}
	if p.bearer != "" {
		h.SetHeaders("Authorization: Bearer " + p.bearer)
	}
	if p.cryptoHex != "" {
		h.SetCryptoKeyHex(p.cryptoHex)
	}
}

func (p *httpClientPool) get() *api.HTTPClient {
	h := <-p.ch
	p.mu.Lock()
	p.configureClient(h)
	p.mu.Unlock()
	return h
}

func (p *httpClientPool) put(h *api.HTTPClient) {
	p.ch <- h
}

func (p *httpClientPool) setBase(base string) {
	p.mu.Lock()
	p.base = base
	for _, h := range p.all {
		h.SetApiBase(base)
	}
	p.mu.Unlock()
}

func (p *httpClientPool) setHeaders(headers ...string) {
	p.mu.Lock()
	p.headers = append([]string{}, headers...)
	for _, h := range p.all {
		for _, hdr := range headers {
			h.SetHeaders(hdr)
		}
	}
	p.mu.Unlock()
}

func (p *httpClientPool) setDebug(debug bool) {
	p.mu.Lock()
	p.debug = debug
	for _, h := range p.all {
		h.SetDebug(debug)
	}
	p.mu.Unlock()
}

func (p *httpClientPool) setBearer(bearer string) {
	p.mu.Lock()
	p.bearer = bearer
	for _, h := range p.all {
		h.SetHeaders("Authorization: Bearer " + bearer)
	}
	p.mu.Unlock()
}

func (p *httpClientPool) setCryptoHex(hex string) {
	p.mu.Lock()
	p.cryptoHex = hex
	for _, h := range p.all {
		h.SetCryptoKeyHex(hex)
	}
	p.mu.Unlock()
}

func (p *httpClientPool) newEphemeral() *api.HTTPClient {
	h := api.NewHTTPClientWithEnv()
	p.mu.Lock()
	p.configureClient(h)
	p.mu.Unlock()
	return h
}

type Client struct {
	pool           *httpClientPool
	hmacKeyHex     string
	user           *api.User
	Token          string
	cryptoPassword string
	CryptoSalt     string
	CryptoHexKey   string
}

func NewClient() *Client {
	return NewClientWithPoolSize(8)
}

func NewClientWithPoolSize(size int) *Client {
	c := &Client{
		pool:       newHTTPClientPool(size),
		hmacKeyHex: "436f6e67726174756c6174696f6e7320494620796f7520676f742054484953206661722121203b2921203a29",
	}
	c.SetDebug(false)
	c.pool.setBase("https://apis.icedrive.net/v3/mobile")
	c.pool.setHeaders("User-Agent: icedrive-ios/2.2.2")
	return c
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
	c.pool.setDebug(debug)
}

func (c *Client) SetCryptoPassword(cryptoPassword string) {
	c.cryptoPassword = cryptoPassword
	if c.CryptoSalt == "" {
		h := c.pool.get()
		_, salt, _ := api.FetchCryptoSaltAndStoredHash(h)
		c.pool.put(h)
		c.CryptoSalt = salt
	}
	hex, _ := api.DeriveCryptoKey(cryptoPassword, c.CryptoSalt)
	c.CryptoHexKey = hex
	c.pool.setCryptoHex(hex)
}

func (c *Client) LoginWithUsernameAndPassword(email, password string) error {
	h := c.pool.get()
	user, err := api.LoginWithUsernameAndPassword(h, email, password, c.hmacKeyHex)
	if err != nil {
		c.pool.put(h)
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
	c.pool.setBearer(c.Token)
	c.pool.put(h)
	return nil
}

func (c *Client) LoginWithBearerToken(token string) error {
	h := c.pool.get()
	user, err := api.LoginWithBearerToken(h, token)
	if err != nil {
		c.pool.put(h)
		return err
	}
	c.user = user
	c.Token = h.GetBearerToken()
	c.pool.setBearer(c.Token)
	c.pool.put(h)
	return nil
}

func (c *Client) ListFolder(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.pool.get()
	resp, err := api.GetCollection(h, folderID, api.CollectionCloud)
	c.pool.put(h)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) ListFolderEncrypted(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.pool.get()
	resp, err := api.GetCollection(h, folderID, api.CollectionCrypto)
	c.pool.put(h)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) ListFolderTrash(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.pool.get()
	resp, err := api.GetCollection(h, folderID, api.CollectionTrash)
	c.pool.put(h)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) ListVersions(item api.Item) ([]api.FileVersion, error) {
	h := c.pool.get()
	vers, err := api.ListVersions(h, item)
	c.pool.put(h)
	return vers, err
}

func (c *Client) CreateFolder(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.pool.get()
	err := api.CreateFolder(h, parentID, name, false)
	c.pool.put(h)
	return err
}

func (c *Client) CreateFolderEncrypted(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.pool.get()
	err := api.CreateFolder(h, parentID, name, true)
	c.pool.put(h)
	return err
}

func (c *Client) UploadFile(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.pool.get()
	_, err := api.UploadFile(h, folderID, fileName)
	c.pool.put(h)
	return err
}

func (c *Client) UploadFileEncrypted(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.pool.get()
	_, err := api.UploadEncryptedFile(h, folderID, fileName, c.CryptoHexKey)
	c.pool.put(h)
	return err
}

func (c *Client) UploadFileWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.pool.newEphemeral()
	return api.NewUploadFileWriter(h, folderID, fileName)
}

func (c *Client) UploadFileEncryptedWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.pool.newEphemeral()
	return api.NewUploadFileEncryptedWriter(h, folderID, fileName, c.CryptoHexKey)
}

func (c *Client) DownloadFile(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	h := c.pool.get()
	err := api.DownloadFile(h, item, destPath, false)
	c.pool.put(h)
	return err
}

func (c *Client) DownloadFileStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	h := c.pool.newEphemeral()
	return api.OpenDownloadStream(h, item, false)
}

func (c *Client) DownloadFileEncrypted(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	h := c.pool.get()
	err := api.DownloadFile(h, item, destPath, true)
	c.pool.put(h)
	return err
}

func (c *Client) DownloadFileEncryptedStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	h := c.pool.newEphemeral()
	return api.OpenDownloadStream(h, item, true)
}

func (c *Client) TrashItem(item api.Item) error {
	h := c.pool.get()
	err := api.TrashAdd(h, item)
	c.pool.put(h)
	return err
}

func (c *Client) TrashEraseAll() error {
	h := c.pool.get()
	err := api.TrashEraseAll(h)
	c.pool.put(h)
	return err
}

func (c *Client) RestoreTrashedItem(item api.Item) error {
	h := c.pool.get()
	err := api.TrashRestore(h, item)
	c.pool.put(h)
	return err
}

func (c *Client) Delete(item api.Item) error {
	h := c.pool.get()
	err := api.Delete(h, item)
	c.pool.put(h)
	return err
}

func (c *Client) Rename(item api.Item, newName string) error {
	h := c.pool.get()
	var err error
	if item.IsFolder == 1 {
		err = api.RenameFolder(h, item, newName)
	} else {
		err = api.RenameFile(h, item, newName, false)
	}
	c.pool.put(h)
	return err
}

func (c *Client) Move(targetFolderID uint64, items ...api.Item) error {
	h := c.pool.get()
	err := api.Move(h, targetFolderID, items...)
	c.pool.put(h)
	return err
}

func (c *Client) GetPlainSize(item api.Item) (int64, error) {
	if err := c.defaultAuthChecks(item.Crypto == 1); err != nil {
		return 0, err
	}
	h := c.pool.get()
	size, err := api.GetPlainSize(h, item, item.Crypto == 1)
	c.pool.put(h)
	return size, err
}
