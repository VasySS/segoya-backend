// Package http provides methods for creating http clients.
package http

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// ErrWrongCredentials is returned when the credentials provided for a proxy are wrong.
var ErrWrongCredentials = errors.New("wrong credentials provided for proxy")

// ErrProxyContextUnsupported indicates a proxy dialer without cancellation support.
var ErrProxyContextUnsupported = errors.New("proxy dialer does not support context")

// NewClient creates a new http client.
func NewClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

// NewClientWithSOCKS5 creates a new http client with a SOCKS5 proxy connection.
func NewClientWithSOCKS5(address, login, password string) (*http.Client, error) {
	if address == "" || login == "" || password == "" {
		return nil, ErrWrongCredentials
	}

	proxyAuth := &proxy.Auth{
		User:     login,
		Password: password,
	}

	dialer, err := proxy.SOCKS5("tcp", address, proxyAuth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("error creating socks5 proxy dialer: %w", err)
	}

	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, ErrProxyContextUnsupported
	}

	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: contextDialer.DialContext,
		},
	}, nil
}
