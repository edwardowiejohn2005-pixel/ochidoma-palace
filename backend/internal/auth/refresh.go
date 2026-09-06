package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// Refresh tokens are opaque random strings, never JWTs — the server stores
// only a SHA-256 hash of each one, so they can be looked up and revoked
// (logout, rotation) without ever persisting the raw secret.

func GenerateRefreshToken() (raw string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
