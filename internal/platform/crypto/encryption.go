package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type Service struct {
	key []byte
}

func New(key string) (*Service, error) {
	if key == "" {
		return &Service{key: nil}, nil
	}
	decoded, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("DATA_ENCRYPTION_KEY must be 32 bytes after decoding")
	}
	return &Service{key: decoded}, nil
}

func (s *Service) Configured() bool {
	return len(s.key) == 32
}

func (s *Service) Encrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	if !s.Configured() {
		return plain, nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	return append(nonce, ciphertext...), nil
}

func (s *Service) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	if !s.Configured() {
		return ciphertext, nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	data := ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

func (s *Service) EncryptString(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	return s.Encrypt([]byte(value))
}

func (s *Service) DecryptString(value []byte) (string, error) {
	plain, err := s.Decrypt(value)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func decodeKey(raw string) ([]byte, error) {
	if len(raw) == 64 {
		decoded, err := hex.DecodeString(raw)
		if err == nil {
			return decoded, nil
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return []byte(raw), nil
}
