package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// tokenBytes is the entropy of session and security tokens (256 bits).
const tokenBytes = 32

// newToken returns a fresh opaque token and its storage hash. The raw token
// goes to the client exactly once; only the hash is persisted.
func newToken() (raw string, hash []byte, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("auth: generate token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

// hashToken returns the SHA-256 digest used to look up a raw token. The
// tokens have 256 bits of entropy, so an unsalted hash is safe: it cannot
// be brute-forced, and identical tokens are never issued.
func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
