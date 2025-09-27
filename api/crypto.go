package api

import (
	"encoding/hex"
	"errors"
	"net/url"

	"golang.org/x/crypto/twofish"
)

const blockSize = 16

// DecryptFilename takes the 64-byte hex key and a hex-encoded filename string
// and returns the decoded plaintext filename.
func DecryptFilename(keyHex string, cipherHex string) (string, error) {
	// Parse hex key
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", errors.New("key must decode to 32 bytes (64 hex chars)")
	}

	// Parse ciphertext
	ct, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	if len(ct)%blockSize != 0 {
		return "", errors.New("ciphertext not multiple of block size")
	}

	// Init cipher
	c, err := twofish.NewCipher(key)
	if err != nil {
		return "", err
	}

	// JS uses a fixed IV "1234567887654321"
	iv := []byte("1234567887654321")
	if len(iv) != blockSize {
		return "", errors.New("bad iv length")
	}

	pt := make([]byte, len(ct))
	prev := iv

	// CBC decrypt
	for i := 0; i < len(ct); i += blockSize {
		block := make([]byte, blockSize)
		c.Decrypt(block, ct[i:i+blockSize])
		for j := 0; j < blockSize; j++ {
			pt[i+j] = block[j] ^ prev[j]
		}
		prev = ct[i : i+blockSize]
	}

	// Strip zero padding
	end := len(pt)
	for end > 0 && pt[end-1] == 0 {
		end--
	}
	trimmed := pt[:end]

	// URL decode (like JS urldecode)
	decoded, err := url.QueryUnescape(string(trimmed))
	if err != nil {
		return string(trimmed), nil // fallback: return raw if urldecode fails
	}
	return decoded, nil
}

// EncryptFilename takes the 64-byte hex key and a plaintext filename string
// and returns the hex-encoded ciphertext.
func EncryptFilename(keyHex string, filename string) (string, error) {
	// Parse hex key
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", errors.New("key must decode to 32 bytes (64 hex chars)")
	}

	// Init cipher
	c, err := twofish.NewCipher(key)
	if err != nil {
		return "", err
	}

	// JS uses a fixed IV "1234567887654321"
	iv := []byte("1234567887654321")
	if len(iv) != blockSize {
		return "", errors.New("bad iv length")
	}

	// URL encode (like JS encodeURI)
	escaped := url.QueryEscape(filename)
	plain := []byte(escaped)

	// Pad with zeros to blocksize
	padLen := blockSize - (len(plain) % blockSize)
	if padLen == blockSize {
		padLen = 0
	}
	for i := 0; i < padLen; i++ {
		plain = append(plain, 0)
	}

	ct := make([]byte, len(plain))
	prev := iv

	// CBC encrypt
	for i := 0; i < len(plain); i += blockSize {
		block := make([]byte, blockSize)
		for j := 0; j < blockSize; j++ {
			block[j] = plain[i+j] ^ prev[j]
		}
		c.Encrypt(ct[i:i+blockSize], block)
		prev = ct[i : i+blockSize]
	}

	return hex.EncodeToString(ct), nil
}
