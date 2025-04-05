package app

import (
	"context"
	"errors"
	"sync"
)

// ErrCloserContextClosed is returned when closer didn't close everything in time.
var ErrCloserContextClosed = errors.New("context closed")

type closeFunc func(ctx context.Context) error

// Closer is a helper for graceful shutdown of created resources.
type Closer struct {
	mu    sync.Mutex
	funcs []closeFunc
}

// NewCloser returns a new Closer, a helper for graceful shutdown of created resources.
//
// Implementation based on:
// https://blog.ildarkarymov.ru/posts/graceful-shutdown/
func NewCloser() *Closer {
	return &Closer{
		funcs: make([]closeFunc, 0),
	}
}

// AddWithCtx adds a function to Closer that needs a context and returns an error.
func (c *Closer) AddWithCtx(f closeFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.funcs = append(c.funcs, f)
}

// Add adds a function to Closer that doesn't need a context and returns no error.
func (c *Closer) Add(f func()) {
	c.AddWithCtx(func(context.Context) error {
		f()
		return nil
	})
}

// AddWithError adds a function to Closer that doesn't need a context and returns an error.
func (c *Closer) AddWithError(f func() error) {
	c.AddWithCtx(func(context.Context) error {
		return f()
	})
}

// Close starts closing all added functions in reverse order.
func (c *Closer) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var combinedErr error

	done := make(chan struct{})
	go func() {
		defer close(done)

		for i := len(c.funcs) - 1; i >= 0; i-- {
			if err := c.funcs[i](ctx); err != nil {
				combinedErr = errors.Join(combinedErr, err)
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ErrCloserContextClosed
	case <-done:
		return combinedErr
	}
}
