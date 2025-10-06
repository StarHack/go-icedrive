package client

import (
	"errors"
	"io"

	"github.com/StarHack/go-icedrive/api"
)

type Client struct {
	httpc          *api.HTTPClient
	hmacKeyHex     string
	user           *api.User
	Token          string
	cryptoPassword string
	CryptoSalt     string
	cryptoHexKey   string
}

func NewClient() *Client {
	var client = Client{httpc: api.NewHTTPClientWithEnv()}
	client.hmacKeyHex = "436f6e67726174756c6174696f6e7320494620796f7520676f742054484953206661722121203b2921203a29"
	client.SetDebug(false)
	client.httpc.SetApiBase("https://apis.icedrive.net/v3/mobile")
	client.httpc.SetHeaders("User-Agent: icedrive-ios/2.8.9")
	return &client
}

func (c *Client) defaultAuthChecks(crypto bool) error {
	if c.user == nil {
		return errors.New("login first")
	}
	if crypto && c.cryptoHexKey == "" {
		return errors.New("set crypto password first")
	}
	return nil
}

func (c *Client) SetDebug(debug bool) {
	c.httpc.SetDebug(debug)
}

func (c *Client) SetCryptoPassword(cryptoPassword string) {
	c.cryptoPassword = cryptoPassword
	if c.CryptoSalt == "" {
		_, salt, _ := api.FetchCryptoSaltAndStoredHash(c.httpc)
		c.CryptoSalt = salt
	}
	c.cryptoHexKey, _ = api.DeriveCryptoKey(cryptoPassword, c.CryptoSalt)
}

func (c *Client) LoginWithUsernameAndPassword(email, password string) error {
	user, err := api.LoginWithUsernameAndPassword(c.httpc, email, password, c.hmacKeyHex)
	if err != nil {
		return err
	}
	c.user = user
	c.Token = c.httpc.GetBearerToken()
	return nil
}

func (c *Client) LoginWithBearerToken(token string) error {
	user, err := api.LoginWithBearerToken(c.httpc, token)
	if err != nil {
		return err
	}
	c.user = user
	c.Token = c.httpc.GetBearerToken()
	return nil
}

func (c *Client) ListFolder(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	response, err := api.GetCollection(c.httpc, folderID, api.CollectionCloud)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderEncrypted(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	response, err := api.GetCollection(c.httpc, folderID, api.CollectionCrypto)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderTrash(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	response, err := api.GetCollection(c.httpc, folderID, api.CollectionTrash)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListVersions(item api.Item) ([]api.FileVersion, error) {
	return api.ListVersions(c.httpc, item)
}

func (c *Client) UploadFile(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	_, err := api.UploadFile(c.httpc, folderID, fileName)
	return err
}

func (c *Client) UploadFileEncrypted(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	_, err := api.UploadEncryptedFile(c.httpc, folderID, fileName, c.cryptoHexKey)
	return err
}

func (c *Client) DownloadFile(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	return api.DownloadFile(c.httpc, item, destPath, false)
}

func (c *Client) DownloadFileStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	return api.OpenDownloadStream(c.httpc, item, false)
}

func (c *Client) DownloadFileEncrypted(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	return api.DownloadFile(c.httpc, item, destPath, true)
}

func (c *Client) DownloadFileEncryptedStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	return api.OpenDownloadStream(c.httpc, item, true)
}

func (c *Client) TrashItem(item api.Item) error {
	return api.TrashAdd(c.httpc, item)
}

func (c *Client) TrashEraseAll() error {
	return api.TrashEraseAll(c.httpc)
}

func (c *Client) RestoreTrashedItem(item api.Item) error {
	return api.TrashRestore(c.httpc, item)
}

func (c *Client) Delete(item api.Item) error {
	return api.Delete(c.httpc, item)
}

func (c *Client) Rename(item api.Item, newName string) error {
	if item.IsFolder == 1 {
		return api.RenameFolder(c.httpc, item, newName)
	} else {
		return api.RenameFile(c.httpc, item, newName, false)
	}
}

func (c *Client) Move(targetFolderID uint64, items ...api.Item) error {
	return api.Move(c.httpc, targetFolderID, items...)
}
