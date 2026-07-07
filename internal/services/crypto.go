package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PasswordHasher defines hashing and verification behaviour.
//
//go:generate mockery --name PasswordHasher --structname PasswordHasherMock --filename password_hasher_mock.go --output mocks --outpkg mocks
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}

// argon2idHasher implements PasswordHasher using Argon2id with OWASP-recommended params.
type argon2idHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
}

// NewArgon2idHasher returns a PasswordHasher with OWASP-recommended Argon2id parameters.
// memory=64 MiB, iterations=3, parallelism=2, salt=32 bytes, key=32 bytes.
func NewArgon2idHasher() PasswordHasher {
	return &argon2idHasher{
		memory:      64 * 1024, // 64 MiB
		iterations:  3,
		parallelism: 2,
		saltLen:     32,
		keyLen:      32,
	}
}

// Hash derives an Argon2id hash from the plain-text password.
// Returned format: argon2id$v=19$m=<m>,t=<t>,p=<p>$<base64-salt>$<base64-hash>
func (h *argon2idHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLen)

	encoded := fmt.Sprintf(
		"argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.memory,
		h.iterations,
		h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// Verify checks whether password matches the stored hash.
func (h *argon2idHasher) Verify(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 {
		return false, fmt.Errorf("invalid hash format")
	}
	// parts: [argon2id, v=19, m=...,t=...,p=..., <salt-b64>, <hash-b64>]

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parsing hash params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, fmt.Errorf("decoding salt: %w", err)
	}

	storedHash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decoding hash: %w", err)
	}

	keyLen := uint32(len(storedHash))
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)

	return subtle.ConstantTimeCompare(computed, storedHash) == 1, nil
}
