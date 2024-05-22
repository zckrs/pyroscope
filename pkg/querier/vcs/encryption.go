package vcs

import (
	"encoding/json"
	"errors"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/encryption"
	"golang.org/x/oauth2"
)

const gcmNonceSize = 12

func encryptToken(token *oauth2.Token, key []byte) ([]byte, error) {
	cipher, err := encryption.NewGCMCipher(key)
	if err != nil {
		return nil, err
	}

	textBytes, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}

	enc, err := cipher.Encrypt(textBytes)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

func decryptToken(encrypted []byte, key []byte) (*oauth2.Token, error) {
	if len(encrypted) < gcmNonceSize {
		return nil, errors.New("malformed token")
	}

	cipher, err := encryption.NewGCMCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := cipher.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	err = json.Unmarshal(plaintext, &token)
	return &token, err
}
