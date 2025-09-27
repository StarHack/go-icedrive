package main

import (
	"fmt"
	"log"
	"os"
	"github.com/StarHack/go-icedrive/api"
)

func main() {
	api.LoadEnvFile(".env")

	// "Od9af1bf0e54ed4c7469741ad2796a7e557f3e973f00ba316b7f63327701a5d3"
	
	res, err := api.EncryptFilename("0d9af1bf0e54ed4c7469741ad2796a7e557f3e973f00ba316b7f63327701a5d3", "Game.of.Thrones.S02E10.Valar.Morghulis.German.DL.1080p.BluRay.iNTERNAL.x264-JaJunge")
	if err != nil {
		panic(err)
	}
	fmt.Println(res)

	res, err = api.DecryptFilename("0d9af1bf0e54ed4c7469741ad2796a7e557f3e973f00ba316b7f63327701a5d3", "2cc3b4ae1cc78e0394ab0d405daab44dc4c4a6aaa71b19321785edc18d9369acdfd27feb9c6dbff7b8863fa1f6516f743b8b7d70ebe11e522cd5ecdbd81c99d880ae7860660795aaefb958804f96a82767559ac5083d3980744518bd4aa77370")
	//res, err := api.DecryptFilename("0d9af1bf0e54ed4c7469741ad2796a7e557f3e973f00ba316b7f63327701a5d3", "1e5043e83ae11476118fac78b537e650ede60bd2b3be26ddec26bb3e928b6a61dbb2aee437573468fc97de0f0f1d761112d4031537ea37607c0b3ddd5a026d31")
	if err != nil {
		panic(err)
	}
	fmt.Println(api.EnvCryptoKey())
	fmt.Println("Result:")
	fmt.Println(res)
	os.Exit(0)


	if api.EnvEmail() == "" && api.EnvPassword() == "" && api.EnvBearer() == "" {
		log.Fatal("ICEDRIVE_EMAIL and ICEDRIVE_PASSWORD OR ICEDRIVE_BEARER must be set")
	}

	client := api.NewHTTPClientWithEnv()

	// Username / Password Login
	/*
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
	*/

	// Login with already known Bearer token
	api.LoginWithBearerToken(client, api.EnvBearer())

}
