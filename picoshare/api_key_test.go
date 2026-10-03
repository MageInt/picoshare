package picoshare_test

import (
	"fmt"
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestAPIKeyFromString(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		input       string
		errExpected error
	}{
		{
			explanation: "a key with the prefix and 40 alphanumeric characters is valid",
			input:       "ps_0123456789abcdefghijABCDEFGHIJ0123456789",
			errExpected: nil,
		},
		{
			explanation: "an empty key is invalid",
			input:       "",
			errExpected: picoshare.ErrInvalidAPIKey,
		},
		{
			explanation: "a key without the prefix is invalid",
			input:       "xx_0123456789abcdefghijABCDEFGHIJ0123456789",
			errExpected: picoshare.ErrInvalidAPIKey,
		},
		{
			explanation: "a key with too few characters is invalid",
			input:       "ps_0123456789abcdefghijABCDEFGHIJ012345678",
			errExpected: picoshare.ErrInvalidAPIKey,
		},
		{
			explanation: "a key with too many characters is invalid",
			input:       "ps_0123456789abcdefghijABCDEFGHIJ01234567890",
			errExpected: picoshare.ErrInvalidAPIKey,
		},
		{
			explanation: "a key with a non-alphanumeric character is invalid",
			input:       "ps_0123456789abcdefghijABCDEFGHIJ012345678-",
			errExpected: picoshare.ErrInvalidAPIKey,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.input), func(t *testing.T) {
			key, err := picoshare.APIKeyFromString(tt.input)
			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", got, want)
			}
			if err != nil {
				return
			}
			if got, want := key.String(), tt.input; got != want {
				t.Errorf("key=%s, want=%s", got, want)
			}
		})
	}
}

func TestNewAPIKey(t *testing.T) {
	key := picoshare.NewAPIKey()

	parsed, err := picoshare.APIKeyFromString(key.String())
	if err != nil {
		t.Fatalf("failed to parse generated API key: %v", err)
	}
	if got, want := parsed, key; got != want {
		t.Errorf("parsed=%v, want=%v", got, want)
	}

	if other := picoshare.NewAPIKey(); other == key {
		t.Errorf("two generated API keys are identical: %s", key)
	}
}

func TestAPIKeyHash(t *testing.T) {
	key, err := picoshare.APIKeyFromString("ps_0123456789abcdefghijABCDEFGHIJ0123456789")
	if err != nil {
		t.Fatalf("failed to parse API key: %v", err)
	}
	otherKey, err := picoshare.APIKeyFromString("ps_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("failed to parse API key: %v", err)
	}

	if !key.Hash().Equal(key.Hash()) {
		t.Errorf("hashes of the same key should be equal")
	}
	if key.Hash().Equal(otherKey.Hash()) {
		t.Errorf("hashes of different keys should not be equal")
	}

	restored, err := picoshare.APIKeyHashFromBytes(key.Hash().Bytes())
	if err != nil {
		t.Fatalf("failed to restore hash from bytes: %v", err)
	}
	if !restored.Equal(key.Hash()) {
		t.Errorf("restored hash should equal the original hash")
	}

	if _, err := picoshare.APIKeyHashFromBytes([]byte("too short")); err == nil {
		t.Errorf("hash with the wrong length should return an error")
	}
}
