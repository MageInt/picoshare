package picoshare

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mtlynch/picoshare/random"
)

const (
	apiKeyPrefix           = "ps_"
	apiKeyRandomPartLength = 40
	apiKeyCharacters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// ErrInvalidAPIKey indicates that text does not have the format of an API key.
var ErrInvalidAPIKey = errors.New("invalid API key format")

// APIKey is a secret that grants access to PicoShare's programmatic API.
type APIKey struct {
	value string
}

// APIKeyHash is the SHA-256 digest of an API key.
type APIKeyHash struct {
	value [sha256.Size]byte
}

// APIKeyPermissions lists the operations an API key allows beyond uploading
// and listing files.
type APIKeyPermissions struct {
	AllowEdit   bool
	AllowDelete bool
}

// APIKeyRecord describes the API key that PicoShare currently accepts.
type APIKeyRecord struct {
	Hash        APIKeyHash
	Created     time.Time
	Permissions APIKeyPermissions
}

// NewAPIKey generates a random API key.
func NewAPIKey() APIKey {
	return APIKey{
		value: apiKeyPrefix + random.String(
			apiKeyRandomPartLength, []rune(apiKeyCharacters)),
	}
}

// APIKeyFromString constructs an API key from user-provided text.
func APIKeyFromString(raw string) (APIKey, error) {
	randomPart, ok := strings.CutPrefix(raw, apiKeyPrefix)
	if !ok || len(randomPart) != apiKeyRandomPartLength {
		return APIKey{}, ErrInvalidAPIKey
	}
	for _, c := range randomPart {
		if !strings.ContainsRune(apiKeyCharacters, c) {
			return APIKey{}, ErrInvalidAPIKey
		}
	}
	return APIKey{value: raw}, nil
}

func (k APIKey) String() string {
	return k.value
}

// Hash returns the SHA-256 digest of the API key. API keys have enough entropy
// that a fast hash resists brute force without a slow KDF.
func (k APIKey) Hash() APIKeyHash {
	return APIKeyHash{value: sha256.Sum256([]byte(k.value))}
}

// APIKeyHashFromBytes constructs an API key hash from a stored digest.
func APIKeyHashFromBytes(b []byte) (APIKeyHash, error) {
	if len(b) != sha256.Size {
		return APIKeyHash{}, fmt.Errorf(
			"API key hash has length %d, want %d", len(b), sha256.Size)
	}
	h := APIKeyHash{}
	copy(h.value[:], b)
	return h, nil
}

// Bytes returns the raw digest.
func (h APIKeyHash) Bytes() []byte {
	return h.value[:]
}

// Equal performs a constant-time comparison between two hashes.
func (h APIKeyHash) Equal(other APIKeyHash) bool {
	return subtle.ConstantTimeCompare(h.value[:], other.value[:]) == 1
}
