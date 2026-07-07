package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCryptoService_Hash(t *testing.T) {
	hasher := NewArgon2idHasher()

	t.Run("hash_succeeds", func(t *testing.T) {
		hash, err := hasher.Hash("my-secret-password")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(hash, "argon2id$"), "hash should have argon2id prefix")
		assert.Equal(t, 5, len(strings.Split(hash, "$")), "hash should have 5 parts")
	})

	t.Run("two_hashes_same_password_have_different_salts", func(t *testing.T) {
		hash1, err1 := hasher.Hash("same-password")
		hash2, err2 := hasher.Hash("same-password")
		require.NoError(t, err1)
		require.NoError(t, err2)
		// Different salts must produce different encoded strings.
		assert.NotEqual(t, hash1, hash2, "hashes of same password must differ due to random salt")
		// But both must verify correctly.
		ok1, _ := hasher.Verify("same-password", hash1)
		ok2, _ := hasher.Verify("same-password", hash2)
		assert.True(t, ok1)
		assert.True(t, ok2)
	})
}

func TestCryptoService_Verify(t *testing.T) {
	hasher := NewArgon2idHasher()
	password := "correct-horse-battery-staple"
	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	type testCase struct {
		name     string
		password string
		hash     string
		wantOK   bool
		wantErr  bool
	}

	tests := []testCase{
		{
			name:     "correct_password_returns_true",
			password: password,
			hash:     hash,
			wantOK:   true,
			wantErr:  false,
		},
		{
			name:     "wrong_password_returns_false",
			password: "wrong-password",
			hash:     hash,
			wantOK:   false,
			wantErr:  false,
		},
		{
			name:     "malformed_hash_returns_error",
			password: password,
			hash:     "notahash",
			wantOK:   false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := hasher.Verify(tt.password, tt.hash)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOK, ok)
			}
		})
	}
}
