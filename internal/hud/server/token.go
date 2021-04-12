package server

import (
	"crypto/rand"
	"encoding/base64"
)

type BearerToken string

// Generate a new bearer token for authenticating against the apiserver. Uses
// 256 bits of entropy and generates a url-safe token.
func NewBearerToken() (BearerToken, error) {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return BearerToken(base64.URLEncoding.EncodeToString(b)), nil
}
