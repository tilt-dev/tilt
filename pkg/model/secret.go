package model

import (
	"bytes"
	"encoding/base64"
	"fmt"
)

// Don't scrub secrets less than 5 characters,
// to avoid weird behavior where we scrub too much
// https://app.clubhouse.io/windmill/story/3568/small-secrets-lead-to-weird-behavior
const minSecretLengthToScrub = 5

// Secrets are different than other kinds of build/deploy outputs.
//
// Once something is marked as a secret, we want to scrub it from all logs
// until the user quits Tilt. Removing the secret from the Kubernetes cluster
// doesn't suddenly "reveal" the secret.
type SecretSet map[string]Secret

func (s SecretSet) AddAll(other SecretSet) {
	for key, val := range other {
		s[key] = val
	}
}

func (s SecretSet) AddSecret(name string, key string, value []byte) {
	v := string(value)
	valueEncoded := base64.StdEncoding.EncodeToString(value)
	s[v] = Secret{
		Name:         name,
		Key:          key,
		Value:        value,
		ValueEncoded: []byte(valueEncoded),
		Replacement:  []byte(fmt.Sprintf("[redacted secret %s:%s]", name, key)),
	}
}

func (s SecretSet) Scrub(text []byte) []byte {
	for _, secret := range s {
		text = secret.Scrub(text)
	}
	return text
}

type Secret struct {
	// The name of the secret in the kubernetes cluster, so the user
	// can look it up themselves.
	Name string

	Key string

	Value []byte

	ValueEncoded []byte

	Replacement []byte
}

func (s Secret) Scrub(text []byte) []byte {
	if len(s.Value) < minSecretLengthToScrub {
		return text
	}
	if bytes.Contains(text, s.Value) {
		text = bytes.ReplaceAll(text, s.Value, s.Replacement)
	}
	if bytes.Contains(text, s.ValueEncoded) {
		text = bytes.ReplaceAll(text, s.ValueEncoded, s.Replacement)
	}
	return text
}
