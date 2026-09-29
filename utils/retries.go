package utils

import (
	"context"
	"fmt"
	"log"
	"time"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Factor       float64
}

// DefaultRetryConfig provides sensible defaults (3 attempts: 100ms, 200ms, 400ms)
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     2 * time.Second,
	Factor:       2.0,
}

// DoWithRetry executes an operation with exponential backoff & context cancellation support
func DoWithRetry(ctx context.Context, name string, cfg RetryConfig, fn func() error) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Run operation
		err := fn()
		if err == nil {
			return nil // Success!
		}

		lastErr = err
		log.Printf("[Retry Warning] Operation '%s' failed (Attempt %d/%d): %v", name, attempt, cfg.MaxAttempts, err)

		if attempt == cfg.MaxAttempts {
			break
		}

		// Wait with context cancellation check
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation '%s' cancelled during retry: %w", name, ctx.Err())
		case <-time.After(delay):
		}

		// Increase delay with exponential backoff
		delay = time.Duration(float64(delay) * cfg.Factor)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return fmt.Errorf("operation '%s' failed after %d attempts: %w", name, cfg.MaxAttempts, lastErr)
}
