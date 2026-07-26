package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
)

func EnsureToken(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		if len(data) < 32 {
			return "", errors.New("the existing API token is invalid")
		}
		return string(data), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", err
	}
	return token, nil
}

func ReadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
