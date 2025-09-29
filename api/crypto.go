package api

import (
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/crypto/twofish"
)

type cryptoAuthResp struct {
	Error  bool   `json:"error"`
	Method string `json:"method"`
	Hash   string `json:"hash"`
}

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

func DecryptTwofishCBCStream(dst io.Writer, src io.Reader, key []byte) error {
	if l := len(key); l != 16 && l != 24 && l != 32 {
		return fmt.Errorf("invalid key length: %d", l)
	}
	block, err := twofish.NewCipher(key)
	if err != nil {
		return err
	}

	const blockSize = 16
	const chunkSize = 4 * 1024 * 1024

	headerCipher := make([]byte, 2*blockSize)
	if _, err := io.ReadFull(src, headerCipher); err != nil {
		return err
	}

	headerIV := []byte("1234567887654321")
	hcbc := cipher.NewCBCDecrypter(block, headerIV)
	headerPlain := make([]byte, len(headerCipher))
	hcbc.CryptBlocks(headerPlain, headerCipher)

	contentIV := headerPlain[:blockSize]
	numPadding := int(headerPlain[blockSize])
	if headerPlain[blockSize+1] != 0 {
		return fmt.Errorf("unsupported file version: %d", headerPlain[blockSize+1])
	}

	newCBC := func() cipher.BlockMode { return cipher.NewCBCDecrypter(block, contentIV) }
	cbc := newCBC()

	chunkRemaining := chunkSize - 2*blockSize

	buf := make([]byte, 128*1024)
	var carry []byte
	var wroteAny bool

	writeOut := func(b []byte, final bool) error {
		if final {
			if numPadding < 0 || numPadding > len(b) {
				return fmt.Errorf("invalid padding")
			}
			b = b[:len(b)-numPadding]
		}
		if len(b) == 0 {
			return nil
		}
		_, err := dst.Write(b)
		if err == nil {
			wroteAny = true
		}
		return err
	}

	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			data := append(carry, buf[:n]...)
			for len(data) >= blockSize {
				toProcess := len(data)
				if toProcess > chunkRemaining {
					toProcess = chunkRemaining
				}
				toProcess = (toProcess / blockSize) * blockSize
				if toProcess == 0 {
					break
				}
				out := make([]byte, toProcess)
				cbc.CryptBlocks(out, data[:toProcess])
				data = data[toProcess:]
				chunkRemaining -= toProcess
				lastRead := rerr == io.EOF && len(data) == 0
				if lastRead {
					if err := writeOut(out, true); err != nil {
						return err
					}
				} else {
					if err := writeOut(out, false); err != nil {
						return err
					}
				}
				if chunkRemaining == 0 && !lastRead {
					cbc = newCBC()
					chunkRemaining = chunkSize
				}
			}
			carry = data
		}

		if rerr != nil {
			if rerr == io.EOF {
				if len(carry) != 0 {
					return io.ErrUnexpectedEOF
				}
				if !wroteAny && numPadding != 0 {
					return fmt.Errorf("unexpected empty body")
				}
				return nil
			}
			return rerr
		}
	}
}

func FetchCryptoSaltAndStoredHash(h *HTTPClient) (storedHex, salt string, err error) {
	if h == nil {
		h = NewHTTPClientWithEnv()
	}
	status, _, body, e := h.httpGET("https://apis.icedrive.net/v3/webapp/crypto-auth")
	if e != nil {
		return "", "", e
	}
	if status >= 400 {
		return "", "", errors.New("crypto-auth failed")
	}
	var rs cryptoAuthResp
	if err := json.Unmarshal(body, &rs); err != nil {
		return "", "", err
	}
	if rs.Error || !strings.HasPrefix(rs.Hash, "ICE::") {
		return "", "", errors.New("bad crypto-auth response")
	}
	t := strings.TrimPrefix(rs.Hash, "ICE::")
	parts := strings.SplitN(t, "::", 2)
	if len(parts) != 2 {
		return "", "", errors.New("unexpected hash format")
	}
	return parts[0], parts[1], nil
}

func DeriveCryptoKey(password, salt string) (string, error) {
	dk, err := pbkdf2.Key(sha1.New, password, []byte(salt), 50000, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(dk), nil
}
