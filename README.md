# go-icedrive

Provides a pure Go client compatible with the Icedrive API.

**Status: WiP, expect breaking changes for now**

Currently supports:

- Login
  - Username/password incl. proof-of-work solution (captcha)
  - Bearer token
- List Folder
- Upload Files
- Download Files
- Move File / Folder to trash
- Empty Trash
- List File Versions

**Encryption**

- Encrypt and Decrypt Filenames
- List Encrypted Folder
- Derive Crypto Hash
- Download Encrypted Files
- Upload Encrypted Files

## Getting Started

Copy `.env-sample` to `.env` and use your own email + password. You may then create `main.go` to implement a client as shown below.

```go
package main

import (
	"fmt"
	"log"
	"github.com/StarHack/go-icedrive/client"
)

func main() {
	c := client.NewClient()
	err := c.LoginWithUsernameAndPassword("your@email.com", "yourpassword")
	if err != nil {
		panic(err)
	}

	// (optional) set crypto password if you want to work with encrypted content
	c.SetCryptoPassword("your-crypto-password")

	// List root folder
	r, err := c.ListFolder(uint64(0))
	if err != nil {
		panic(err)
	}

	// Find hello-world.txt and download it
	for _, item := range r {
		fmt.Printf("%s (%s, %v)\n", item.Filename, item.UID, item.ID)
		if item.Filename == "hello-world.txt" {
			err = c.DownloadFile(item, "downloads/")
			if err != nil {
				panic(err)
			}
			fmt.Println("download successful!")
		}
	}
}

```
