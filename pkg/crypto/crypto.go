// Package crypto contains methods for working with UUIDs, random strings, etc.
package crypto

// Service is a crypto service for working with UUIDs, random strings, etc.
type Service struct{}

// NewService creates new crypto service.
func NewService() *Service {
	return &Service{}
}
