package main

import (
	"fmt"
	"log"
	"os"

	"icedrive/api"
)

func main() {
	api.LoadEnvFile(".env")

	email := api.EnvEmail()
	password := api.EnvPassword()
	if email == "" || password == "" {
		log.Fatal("ICEDRIVE_EMAIL and ICEDRIVE_PASSWORD must be set")
	}

	client := api.NewHTTPClientWithEnv()

	status, _, body, err := api.Login(client, email, password, "436f6e67726174756c6174696f6e7320494620796f7520676f742054484953206661722121203b2921203a29")
	if err != nil {
		if len(body) > 0 {
			os.Stdout.Write(body)
			fmt.Println()
		}
		log.Fatalf("login request error: %v", err)
	}

	fmt.Println("status:", status)
	fmt.Println(string(body))
}
