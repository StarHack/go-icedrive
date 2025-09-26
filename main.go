package main

import (
	"fmt"
	"log"

	"icedrive/api"
)

func main() {
	api.LoadEnvFile(".env")

	if api.EnvEmail() == "" && api.EnvPassword() == "" && api.EnvBearer() == "" {
		log.Fatal("ICEDRIVE_EMAIL and ICEDRIVE_PASSWORD OR ICEDRIVE_BEARER must be set")
	}

	client := api.NewHTTPClientWithEnv()

	/*
	status, _, body, err := api.Login(client, api.EnvEmail(), api.EnvPassword(), api.EnvHmac())
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

	api.LoginWithBearerToken(client, api.EnvBearer())


	res, err := api.UploadFile(client, int64(0), "hello-world.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println(res)
}
