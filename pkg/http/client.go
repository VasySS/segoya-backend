// Package http provides methods for creating http clients.
package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// ErrWrongCredentials is returned when the credentials for proxy are wrong.
var ErrWrongCredentials = errors.New("wrong credentials")

// NewClient creates a new http client.
func NewClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

// NewClientWithProxy creates a new http client with a proxy.
func NewClientWithProxy(address, login, password string) (*http.Client, error) {
	if address == "" || login == "" || password == "" {
		return nil, ErrWrongCredentials
	}

	proxyAuth := &proxy.Auth{
		User:     login,
		Password: password,
	}

	dialer, err := proxy.SOCKS5("tcp", address, proxyAuth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("error creating proxy dialer: %w", err)
	}

	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
	}, nil
}
