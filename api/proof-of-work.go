package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// POWChallenge represents the response from /pow-new endpoint
type POWChallenge struct {
	Challenge      string `json:"challenge"`
	DifficultyBits int    `json:"difficultyBits"`
	Exp            uint64 `json:"exp"`
	Scope          string `json:"scope"`
	Token          string `json:"token"`
}

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

// FetchPOWChallenge fetches a new proof-of-work challenge from the API
func FetchPOWChallenge(h *HTTPClient, scope string) (*POWChallenge, error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}

	// Prepare form data
	payload := "app=ios&request=pow-new&scope=" + scope
	code, _, body, err := h.httpPOST("/api", "application/x-www-form-urlencoded", []byte(payload))
	if err != nil {
		return nil, err
	}
	if code < 200 || code >= 400 {
		return nil, fmt.Errorf("failed to fetch POW challenge: HTTP %d, body: %s", code, string(body))
	}
	var challenge POWChallenge
	if err := json.Unmarshal(body, &challenge); err != nil {
		return nil, err
	}
	return &challenge, nil
}

// SolvePOWChallenge solves a proof-of-work challenge by finding a nonce
// 1. Decode challenge from base64
// 2. Append random bytes (8-24 bytes)
// 3. Append 4-byte counter (starts at 0, increments)
// 4. Hash the entire byte array with SHA-256
// 5. Check leading zero bits
// Returns: base64-encoded nonce bytes, hex hash, error
func SolvePOWChallenge(challenge *POWChallenge) (string, string, error) {
	if challenge.DifficultyBits <= 0 || challenge.DifficultyBits > 256 {
		return "", "", fmt.Errorf("invalid difficulty bits: %d", challenge.DifficultyBits)
	}

	challengeBytes, err := base64.RawURLEncoding.DecodeString(challenge.Challenge)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode challenge: %w", err)
	}

	nonceSize := 12
	nonceBytes := make([]byte, nonceSize)
	_, err = rand.Read(nonceBytes)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	bufSize := len(challengeBytes) + nonceSize + 4
	buf := make([]byte, bufSize)
	copy(buf, challengeBytes)
	copy(buf[len(challengeBytes):], nonceBytes)

	counterOffset := len(challengeBytes) + nonceSize

	// Brute force (proof of work): increment counter until we find valid hash
	var counter uint32
	for {
		buf[counterOffset] = byte(counter >> 24)
		buf[counterOffset+1] = byte(counter >> 16)
		buf[counterOffset+2] = byte(counter >> 8)
		buf[counterOffset+3] = byte(counter)

		h := sha256.Sum256(buf)

		if countLeadingZeroBits(h[:]) >= challenge.DifficultyBits {
			nonceResult := make([]byte, nonceSize+4)
			copy(nonceResult, nonceBytes)
			nonceResult[nonceSize] = byte(counter >> 24)
			nonceResult[nonceSize+1] = byte(counter >> 16)
			nonceResult[nonceSize+2] = byte(counter >> 8)
			nonceResult[nonceSize+3] = byte(counter)

			nonceB64 := base64.RawURLEncoding.EncodeToString(nonceResult)
			hashHex := hex.EncodeToString(h[:])
			return nonceB64, hashHex, nil
		}

		counter++
		if counter == 0 {
			return "", "", fmt.Errorf("counter overflow without finding solution")
		}
	}
}

// countLeadingZeroBits counts the number of leading zero bits in a byte array
func countLeadingZeroBits(data []byte) int {
	count := 0
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == 0 {
			count += 8
		} else {
			for bit := 7; bit >= 0; bit-- {
				if (b>>bit)&1 == 0 {
					count++
				} else {
					return count
				}
			}
		}
	}
	return count
}
