# go-icedrive

My simple approach to implementing the Icedrive API.

**Status: WiP, expect breaking changes for now**

Currently supports:

- Login
  - Username/password incl. proof-of-work solution (captcha)
  - Bearer token
- List Folder
- Upload File (encrypted/unencrypted)
- Download File (encrypted/unencrypted)
- Move File / Folder to trash
- Empty Trash

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
	"github.com/StarHack/go-icedrive/api"
)

func main() {
	api.LoadEnvFile(".env")

	if api.EnvEmail() == "" && api.EnvPassword() == "" && api.EnvBearer() == "" {
		log.Fatal("ICEDRIVE_EMAIL and ICEDRIVE_PASSWORD OR ICEDRIVE_BEARER must be set")
	}

	client := api.NewHTTPClientWithEnv()

	// Username / Password Login
	status, _, body, err := api.LoginWithUsernameAndPassword(client, api.EnvEmail(), api.EnvPassword(), api.EnvHmac())
	if err != nil {
		if len(body) > 0 {
			os.Stdout.Write(body)
			fmt.Println()
		}
		log.Fatalf("login request error: %v", err)
	}

	fmt.Println("status:", status)
	fmt.Println(string(body))

	// Login with already known Bearer token instead
	// api.LoginWithBearerToken(client, api.EnvBearer())


	res, err := api.GetCollection(client, int64(0))
	if err != nil {
		panic(err)
	}
	fmt.Println(res)

	err = api.DownloadFile(client, "file-3341885801", "downloads/hello-world.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println("download ok")
}

```
