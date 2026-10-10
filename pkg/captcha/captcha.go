package captcha

import "errors"

var (
	// ErrTokenIsNotProvided is returned when a CAPTCHA token is empty or whitespace-only.
	ErrTokenIsNotProvided = errors.New("token is not provided")
	// ErrVerificationFailed is returned when the provider rejects verification.
	ErrVerificationFailed = errors.New("token did not pass captcha verification")
)
