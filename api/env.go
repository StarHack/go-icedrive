package api

import (
	"bufio"
	"os"
	"strings"
)

func LoadEnvFile(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		os.Setenv(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
}

func EnvEmail() string {
	return os.Getenv("ICEDRIVE_EMAIL")
}

func EnvPassword() string {
	return os.Getenv("ICEDRIVE_PASSWORD")
}

func EnvBearer() string {
	return os.Getenv("ICEDRIVE_BEARER")
}

func EnvAPIHeaders() string {
	return os.Getenv("ICEDRIVE_API_HEADERS")
}

func EnvCookie() string {
	return os.Getenv("ICEDRIVE_COOKIE")
}

