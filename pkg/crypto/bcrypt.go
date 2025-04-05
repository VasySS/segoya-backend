package crypto

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// CompareHashAndPassword compares a hash with a password.
func (s *Service) CompareHashAndPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("error comparing hash and password: %w", err)
	}

	return nil
}

// GenerateHashFromPassword generates a hash from a password.
func (s *Service) GenerateHashFromPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error generating hash: %w", err)
	}

	return string(hash), nil
}
