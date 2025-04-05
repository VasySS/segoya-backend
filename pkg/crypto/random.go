package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

func newRandomBytesArray(length int) []byte {
	b := make([]byte, length)
	_, _ = rand.Read(b)

	return b
}

// NewUUID4 returns a new UUID of version 4.
func (s *Service) NewUUID4() string {
	return uuid.NewString()
}

// NewUUID7 returns a new UUID of version 7.
func (s *Service) NewUUID7() string {
	return uuid.Must(uuid.NewV7()).String()
}

// IsUUIDValid checks if UUID is valid.
func (s *Service) IsUUIDValid(uuidStr string) error {
	if err := uuid.Validate(uuidStr); err != nil {
		return fmt.Errorf("error validating uuid: %w", err)
	}

	return nil
}

// NewRandomHexString returns a random hex string of a given length.
func (s *Service) NewRandomHexString(length int) string {
	return hex.EncodeToString(newRandomBytesArray(length / 2))
}

// NewRandomBase64String returns a random base64 string of a given length.
func (s *Service) NewRandomBase64String(length int) string {
	return base64.StdEncoding.EncodeToString(newRandomBytesArray(length))
}
