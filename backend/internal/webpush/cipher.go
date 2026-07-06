package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// hkdfInfo domain-separates the derived encryption key so the master key
// could be reused for other purposes without key overlap.
var hkdfInfo = []byte("mailfold-webpush-mailbox-password-aes256-gcm-v1")

// Cipher encrypts and decrypts a subscription's stored mailbox password with
// AES-256-GCM. The AES key is HKDF-SHA256-derived from the operator's master
// key, mirroring internal/admin.Cipher, internal/apikey.Cipher, and
// internal/sessionstore.Cipher exactly — each package that persists a secret
// owns its own small cipher rather than sharing one.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher derives the AES-256-GCM key from masterKey (which must be at
// least 32 bytes) and returns a ready cipher.
func NewCipher(masterKey []byte) (*Cipher, error) {
	if len(masterKey) < 32 {
		return nil, errors.New("webpush: master key must be at least 32 bytes")
	}
	key := hkdfExpand(masterKey, hkdfInfo, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext, returning the ciphertext and the fresh random
// nonce used.
func (c *Cipher) Seal(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, c.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = c.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Open decrypts a ciphertext produced by Seal with its nonce.
func (c *Cipher) Open(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("webpush: wrong nonce size")
	}
	return c.aead.Open(nil, nonce, ciphertext, nil)
}

// hkdfExpand is a minimal HKDF-SHA256 (RFC 5869) with a zero salt.
func hkdfExpand(master, info []byte, n int) []byte {
	extractor := hmac.New(sha256.New, make([]byte, sha256.Size))
	extractor.Write(master)
	prk := extractor.Sum(nil)

	var out, prev []byte
	for counter := byte(1); len(out) < n; counter++ {
		h := hmac.New(sha256.New, prk)
		h.Write(prev)
		h.Write(info)
		h.Write([]byte{counter})
		prev = h.Sum(nil)
		out = append(out, prev...)
	}
	return out[:n]
}
