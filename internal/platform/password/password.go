// Package password hashes and verifies passwords with Argon2id.
//
// Hashes use the PHC string format and carry their own parameters, so the
// cost can be raised later while old hashes keep verifying with the
// parameters they were created with.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the Argon2id cost parameters.
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
	KeyLen    uint32
	SaltLen   uint32
}

// DefaultParams follow the OWASP recommendation (19 MiB, 2 iterations, 1 lane).
var DefaultParams = Params{
	MemoryKiB: 19 * 1024,
	Time:      2,
	Threads:   1,
	KeyLen:    32,
	SaltLen:   16,
}

// Password length bounds in bytes. The minimum favours length over
// composition rules; the maximum bounds hashing cost.
const (
	MinLength = 12
	MaxLength = 1024
)

// ErrInvalidHash is returned when a stored hash cannot be parsed.
var ErrInvalidHash = errors.New("password: invalid hash format")

// Hash derives an Argon2id hash of password with DefaultParams.
func Hash(password string) (string, error) {
	return HashWithParams(password, DefaultParams)
}

// HashWithParams derives an Argon2id hash with explicit parameters.
func HashWithParams(password string, p Params) (string, error) {
	if len(password) < MinLength || len(password) > MaxLength {
		return "", fmt.Errorf("password: length must be between %d and %d bytes", MinLength, MaxLength)
	}
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify reports whether password matches the stored hash. The comparison
// is constant-time over the derived keys. A malformed hash returns
// ErrInvalidHash so it can be distinguished from a wrong password.
func Verify(hash, password string) (bool, error) {
	p, salt, key, err := decode(hash)
	if err != nil {
		return false, err
	}
	derived := argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(derived, key) == 1, nil
}

// NeedsRehash reports whether hash was created with weaker parameters than
// DefaultParams and should be upgraded on next successful login.
func NeedsRehash(hash string) bool {
	p, _, key, err := decode(hash)
	if err != nil {
		return true
	}
	return p.MemoryKiB < DefaultParams.MemoryKiB ||
		p.Time < DefaultParams.Time ||
		p.Threads < DefaultParams.Threads ||
		uint32(len(key)) < DefaultParams.KeyLen
}

func decode(hash string) (Params, []byte, []byte, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, ErrInvalidHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	// Refuse degenerate parameters rather than deriving with them.
	if p.MemoryKiB < 1024 || p.Time < 1 || p.Threads < 1 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	return p, salt, key, nil
}

// DummyHash is a valid hash of a random password. Login verifies against it
// when the email is unknown so that the unknown-email path costs the same
// key derivation as a wrong password, which keeps response timing from
// revealing whether an account exists.
var DummyHash = func() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("password: crypto/rand unavailable: %v", err))
	}
	h, err := Hash(base64.RawStdEncoding.EncodeToString(buf))
	if err != nil {
		panic(err)
	}
	return h
}()
