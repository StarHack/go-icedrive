package api

import (
	"bufio"
	"os"
	"strings"
)

var envValues = make(map[string]string)

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
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		envValues[key] = val
	}
}

func EnvEmail() string {
	return envValues["ICEDRIVE_EMAIL"]
}

func EnvPassword() string {
	return envValues["ICEDRIVE_PASSWORD"]
}

func EnvBearer() string {
	return envValues["ICEDRIVE_BEARER"]
}

func EnvHmac() string {
	return envValues["ICEDRIVE_HMAC"]
}

func EnvCryptoKey() string {
	return envValues["ICEDRIVE_CRYPTO_KEY"]
}

func EnvCryptoKey64() string {
	return envValues["ICEDRIVE_CRYPTO_KEY_64"]
}

func EnvAPIHeaders() string {
	return envValues["ICEDRIVE_API_HEADERS"]
}

func EnvCookie() string {
	return envValues["ICEDRIVE_COOKIE"]
}
