package client

import (
	"errors"
	"fmt"
	"io"

	"github.com/StarHack/go-icedrive/api"
)

// pooledWriter wraps an io.WriteCloser and releases the HTTPClient back to the pool on Close
type pooledWriter struct {
	writer io.WriteCloser
	pool   *api.HTTPClientPool
	client *api.HTTPClient
}

func (pw *pooledWriter) Write(p []byte) (int, error) {
	return pw.writer.Write(p)
}

func (pw *pooledWriter) Close() error {
	err := pw.writer.Close()
	pw.pool.Release(pw.client)
	return err
}

// pooledReader wraps an io.ReadCloser and releases the HTTPClient back to the pool on Close
type pooledReader struct {
	reader io.ReadCloser
	pool   *api.HTTPClientPool
	client *api.HTTPClient
}

func (pr *pooledReader) Read(p []byte) (int, error) {
	return pr.reader.Read(p)
}

func (pr *pooledReader) Close() error {
	err := pr.reader.Close()
	pr.pool.Release(pr.client)
	return err
}

type Client struct {
	pool           *api.HTTPClientPool
	hmacKeyHex     string
	user           *api.User
	cryptoPassword string
	CryptoSalt     string
	CryptoHexKey   string

	// Store credentials for automatic re-login
	email    string
	password string
}

func NewClient() *Client {
	defaultConcurrentConnections := 3
	defaultRequestsPerMinute := 200.0
	return NewClientWithPoolSize(defaultConcurrentConnections, defaultRequestsPerMinute)
}

func NewClientWithPoolSize(poolSize int, requestsPerMinute float64) *Client {
	pool := api.NewHTTPClientPool(poolSize, requestsPerMinute)
	client := &Client{
		pool:       pool,
		hmacKeyHex: "436f6e67726174756c6174696f6e7320494620796f7520676f742054484953206661722121203b2921203a29",
	}
	client.SetDebug(false)
	pool.SetApiBase("https://apis.icedrive.net/v3/mobile")
	pool.SetHeaders("User-Agent: icedrive-ios/2.3.1")
	return client
}

func (c *Client) defaultAuthChecks(crypto bool) error {
	if c.user != nil {
		if crypto && c.CryptoHexKey == "" {
			return errors.New("set crypto password first")
		}
		return nil
	}

	if c.email != "" && c.password != "" {
		if crypto && c.CryptoHexKey == "" {
			return errors.New("set crypto password first")
		}
		return nil
	}

	return errors.New("login first")
}

func (c *Client) SetDebug(debug bool) {
	c.pool.SetDebug(debug)
}

func (c *Client) SetCryptoPassword(cryptoPassword string) {
	c.cryptoPassword = cryptoPassword
	if c.CryptoSalt == "" {
		// Acquire a client to fetch crypto salt
		err := c.pool.WithClient(func(h *api.HTTPClient) error {
			_, salt, err := api.FetchCryptoSaltAndStoredHash(h)
			c.CryptoSalt = salt
			return err
		})
		if err != nil {
			// Handle error silently for backward compatibility
			_ = err
		}
	}
	c.CryptoHexKey, _ = api.DeriveCryptoKey(cryptoPassword, c.CryptoSalt)
	c.pool.SetCryptoKeyHex(c.CryptoHexKey)
}

func (c *Client) LoginWithUsernameAndPassword(email, password string) error {
	wasDebug := c.pool.GetDebug()
	if wasDebug {
		fmt.Printf(">>> 🔑 Logging in with username and password...\n")
	}

	// Temporarily enable debug for login to see what's happening
	c.pool.SetDebug(true)
	defer c.pool.SetDebug(wasDebug)

	var user *api.User
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var loginErr error
		user, loginErr = api.LoginWithUsernameAndPassword(h, email, password, c.hmacKeyHex)
		return loginErr
	})
	if err != nil {
		return err
	}
	c.user = user

	c.email = email
	c.password = password

	c.pool.SetReloginFunc(c.relogin)

	if wasDebug {
		fmt.Printf(">>> ✅ Login successful!\n")
	}
	return nil
}

func (c *Client) LoginWithBearerToken(token string) error {
	if c.pool.GetDebug() {
		fmt.Println(">>> 🔑 Logging in with bearer token...")
	}
	var user *api.User
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var loginErr error
		user, loginErr = api.LoginWithBearerToken(h, token)
		return loginErr
	})
	if err != nil {
		return err
	}
	c.user = user

	// If we have credentials, set up re-login
	if c.email != "" && c.password != "" {
		c.pool.SetReloginFunc(c.relogin)
	}

	if c.pool.GetDebug() {
		fmt.Printf(">>> ✅ Login successful!\n")
	}
	return nil
}

// SetCredentials stores login credentials for automatic re-login
// This should be called after LoginWithBearerToken if you want automatic re-login capability
func (c *Client) SetCredentials(email, password string) {
	c.email = email
	c.password = password
	if c.user != nil {
		c.pool.SetReloginFunc(c.relogin)
	}
}

func (c *Client) relogin() error {
	if c.email == "" || c.password == "" {
		return fmt.Errorf("no credentials available for re-login")
	}

	fmt.Println(">>> 🔄 Starting re-login process...")

	// Create a standalone HTTP client for re-login to avoid circular dependency
	// (relogin is called from within an HTTPClient that's already acquired from the pool)
	h := api.NewHTTPClientWithEnv()
	h.SetApiBase(c.pool.GetApiBase())
	h.SetHeaders(c.pool.GetHeaders())
	h.SetDebug(c.pool.GetDebug())
	// Explicitly do NOT set relogin func on this client to avoid infinite recursion

	var newUser *api.User
	var loginErr error
	newUser, loginErr = api.LoginWithUsernameAndPassword(h, c.email, c.password, c.hmacKeyHex)
	if loginErr != nil {
		fmt.Printf(">>> ❌ Re-login failed: %v\n", loginErr)
		return loginErr
	}

	// Update the pool's bearer token so all clients get the new token
	c.pool.SetBearerToken(h.GetBearerToken())
	c.user = newUser
	fmt.Println(">>> ✅ Re-login succeeded!")
	return nil
}

func (c *Client) GetToken() string {
	return c.pool.GetBearerToken()
}

func (c *Client) SetToken(token string) {
	c.pool.SetBearerToken(token)
}

func (c *Client) ListFolder(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	var response *api.CollectionResponse
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var collErr error
		response, collErr = api.GetCollection(h, folderID, api.CollectionCloud)
		return collErr
	})
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderEncrypted(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	var response *api.CollectionResponse
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var collErr error
		response, collErr = api.GetCollection(h, folderID, api.CollectionCrypto)
		return collErr
	})
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) ListFolderTrash(folderID uint64) ([]api.Item, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	var response *api.CollectionResponse
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var collErr error
		response, collErr = api.GetCollection(h, folderID, api.CollectionTrash)
		return collErr
	})
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) GetFolderProperties(folderUID string, crypto bool) (*api.FolderPropertiesResponse, error) {
	if err := c.defaultAuthChecks(crypto); err != nil {
		return nil, err
	}
	var response *api.FolderPropertiesResponse
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var propErr error
		response, propErr = api.GetFolderProperties(h, folderUID, crypto)
		return propErr
	})
	return response, err
}

func (c *Client) ListVersions(item api.Item) ([]api.FileVersion, error) {
	var versions []api.FileVersion
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var listErr error
		versions, listErr = api.ListVersions(h, item)
		return listErr
	})
	return versions, err
}

func (c *Client) CreateFolder(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.CreateFolder(h, parentID, name, false)
	})
}

func (c *Client) CreateFolderEncrypted(parentID uint64, name string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.CreateFolder(h, parentID, name, true)
	})
}

func (c *Client) UploadFile(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		_, err := api.UploadFile(h, folderID, fileName)
		return err
	})
}

func (c *Client) UploadFileEncrypted(folderID uint64, fileName string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		_, err := api.UploadEncryptedFile(h, folderID, fileName, c.CryptoHexKey)
		return err
	})
}

func (c *Client) UploadFileWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	// Note: Writers require a dedicated client that won't be released until Close()
	client := c.pool.Acquire()
	writer, err := api.NewUploadFileWriter(client, folderID, fileName)
	if err != nil {
		c.pool.Release(client)
		return nil, err
	}
	// Wrap the writer to release the client when done
	return &pooledWriter{writer: writer, pool: c.pool, client: client}, nil
}

func (c *Client) UploadFileEncryptedWriter(folderID uint64, fileName string) (io.WriteCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	// Note: Writers require a dedicated client that won't be released until Close()
	client := c.pool.Acquire()
	writer, err := api.NewUploadFileEncryptedWriter(client, folderID, fileName, c.CryptoHexKey)
	if err != nil {
		c.pool.Release(client)
		return nil, err
	}
	// Wrap the writer to release the client when done
	return &pooledWriter{writer: writer, pool: c.pool, client: client}, nil
}

func (c *Client) DownloadFile(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(false); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.DownloadFile(h, item, destPath, false)
	})
}

func (c *Client) DownloadFileStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	// Streams require a dedicated client that won't be released until Close()
	client := c.pool.Acquire()
	reader, err := api.OpenDownloadStream(client, item, false)
	if err != nil {
		c.pool.Release(client)
		return nil, err
	}
	return &pooledReader{reader: reader, pool: c.pool, client: client}, nil
}

func (c *Client) DownloadFileEncrypted(item api.Item, destPath string) error {
	if err := c.defaultAuthChecks(true); err != nil {
		return err
	}
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.DownloadFile(h, item, destPath, true)
	})
}

func (c *Client) DownloadFileEncryptedStream(item api.Item) (io.ReadCloser, error) {
	if err := c.defaultAuthChecks(true); err != nil {
		return nil, err
	}
	// Streams require a dedicated client that won't be released until Close()
	client := c.pool.Acquire()
	reader, err := api.OpenDownloadStream(client, item, true)
	if err != nil {
		c.pool.Release(client)
		return nil, err
	}
	return &pooledReader{reader: reader, pool: c.pool, client: client}, nil
}

func (c *Client) TrashItem(item api.Item) error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.TrashAdd(h, item)
	})
}

func (c *Client) TrashEraseAll() error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.TrashEraseAll(h)
	})
}

func (c *Client) RestoreTrashedItem(item api.Item) error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.TrashRestore(h, item)
	})
}

func (c *Client) Delete(item api.Item) error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.Delete(h, item)
	})
}

func (c *Client) Rename(item api.Item, newName string) error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		if item.IsFolder == 1 {
			return api.RenameFolder(h, item, newName)
		} else {
			return api.RenameFile(h, item, newName, false)
		}
	})
}

func (c *Client) Move(targetFolderID uint64, items ...api.Item) error {
	return c.pool.WithClient(func(h *api.HTTPClient) error {
		return api.Move(h, targetFolderID, items...)
	})
}

func (c *Client) GetPlainSize(item api.Item) (int64, error) {
	if err := c.defaultAuthChecks(item.Crypto == 1); err != nil {
		return 0, err
	}
	var size int64
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var sizeErr error
		size, sizeErr = api.GetPlainSize(h, item, item.Crypto == 1)
		return sizeErr
	})
	return size, err
}

func (c *Client) GetUserStats() (*api.UserStats, error) {
	if err := c.defaultAuthChecks(false); err != nil {
		return nil, err
	}
	var stats *api.UserStats
	err := c.pool.WithClient(func(h *api.HTTPClient) error {
		var statsErr error
		stats, statsErr = api.GetUserStats(h)
		return statsErr
	})
	return stats, err
}
