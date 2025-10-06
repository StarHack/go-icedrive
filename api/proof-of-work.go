package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// Payload is the inner proof-of-work payload.
type Payload struct {
	Challenge  string `json:"challenge"`
	Nonce      uint64 `json:"nonce"`
	Hash       string `json:"hash"`
	Difficulty int    `json:"difficulty"`
	Expires    uint64 `json:"expires"`
}

// Final is the wrapper with signature.
type Final struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"`
}

// sha256Hex returns hex-encoded SHA-256 digest of a string.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// stableStringifyPayload sorts keys and stringifies payload deterministically.
func stableStringifyPayload(p Payload) (string, error) {
	m := map[string]interface{}{
		"challenge":  p.Challenge,
		"difficulty": p.Difficulty,
		"expires":    p.Expires,
		"hash":       p.Hash,
		"nonce":      p.Nonce,
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := []byte{'{'}
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(m[k])
		b = append(b, kb...)
		b = append(b, ':')
		b = append(b, vb...)
		if i != len(keys)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b), nil
}

// hmacSHA256Hex computes HMAC-SHA256 over message using hex key.
func hmacSHA256Hex(message, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(message))
	return hex.EncodeToString(m.Sum(nil)), nil
}

// ComputeProofOfWork computes the form_secure string given server timestamp and key.
func ComputeProofOfWork(serverTimeSec uint64, hmacKeyHex string) (string, error) {
	serverTimeMs := serverTimeSec * 1000
	expires := serverTimeMs + 60000
	challenge := "proof-of-work"
	difficulty := 4

	// brute-force nonce until hash has required prefix
	prefix := strings.Repeat("0", difficulty)
	var nonce uint64
	var hash string
	for {
		hash = sha256Hex(challenge + strconv.FormatUint(nonce, 10))
		if strings.HasPrefix(hash, prefix) {
			break
		}
		nonce++
	}

	payload := Payload{
		Challenge:  challenge,
		Nonce:      nonce,
		Hash:       hash,
		Difficulty: difficulty,
		Expires:    expires,
	}

	stable, err := stableStringifyPayload(payload)
	if err != nil {
		return "", err
	}
	sig, err := hmacSHA256Hex(stable, hmacKeyHex)
	if err != nil {
		return "", err
	}

	final := Final{
		Payload:   payload,
		Signature: sig,
	}
	j, err := json.Marshal(final)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(j), nil
}
