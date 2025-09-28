package api

import (
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return errors.New("invalid key size")
	}
	block, err := twofish.NewCipher(key)
	if err != nil {
		return err
	}
	bs := block.BlockSize()

	hdrCt := make([]byte, 2*bs)
	if _, err := io.ReadFull(src, hdrCt); err != nil {
		return err
	}

	fixedIV := []byte("1234567887654321")
	prev := make([]byte, bs)
	copy(prev, fixedIV)

	hdrPt := make([]byte, 2*bs)
	for off := 0; off < 2*bs; off += bs {
		tmp := make([]byte, bs)
		block.Decrypt(tmp, hdrCt[off:off+bs])
		for i := 0; i < bs; i++ {
			hdrPt[off+i] = tmp[i] ^ prev[i]
		}
		copy(prev, hdrCt[off:off+bs])
	}

	streamIV := make([]byte, bs)
	copy(streamIV, hdrPt[:bs])
	padCount := int(hdrPt[bs])

	copy(prev, streamIV)

	buf := make([]byte, 1<<20)
	var rem []byte
	var outHold []byte

	flush := func(p []byte) error {
		if padCount == 0 {
			_, err := dst.Write(p)
			return err
		}
		outHold = append(outHold, p...)
		if len(outHold) > padCount {
			w := len(outHold) - padCount
			if w > 0 {
				if _, err := dst.Write(outHold[:w]); err != nil {
					return err
				}
				outHold = append([]byte(nil), outHold[w:]...)
			}
		}
		return nil
	}

	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			chunk := append(rem, buf[:n]...)
			blocks := (len(chunk) / bs) * bs
			toProc := blocks
			if rerr == nil {
				if blocks >= bs {
					toProc = blocks - bs
				} else {
					toProc = 0
				}
			}
			if toProc > 0 {
				for off := 0; off < toProc; off += bs {
					ct := chunk[off : off+bs]
					pt := make([]byte, bs)
					block.Decrypt(pt, ct)
					for i := 0; i < bs; i++ {
						pt[i] ^= prev[i]
					}
					if err := flush(pt); err != nil {
						return err
					}
					copy(prev, ct)
				}
			}
			rem = chunk[toProc:]
			if rerr == io.EOF && len(rem) > 0 {
				if len(rem)%bs != 0 {
					return errors.New("trailing bytes not multiple of block size")
				}
				for off := 0; off < len(rem); off += bs {
					ct := rem[off : off+bs]
					pt := make([]byte, bs)
					block.Decrypt(pt, ct)
					for i := 0; i < bs; i++ {
						pt[i] ^= prev[i]
					}
					if err := flush(pt); err != nil {
						return err
					}
					copy(prev, ct)
				}
			}
		}
		if rerr == io.EOF {
			if padCount == 0 && len(outHold) > 0 {
				if _, err := dst.Write(outHold); err != nil {
					return err
				}
			}
			return nil
		}
		if rerr != nil {
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
