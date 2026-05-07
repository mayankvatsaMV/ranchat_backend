// helpers.go — Small utility helpers used across the codebase.
package utils

import (
	"context"
	"encoding/json"
)

// BackgroundContext returns a non-cancellable context for use in goroutines
// that outlive request lifetimes (e.g. WebSocket message handlers).
func BackgroundContext() context.Context {
	return context.Background()
}

// ParseJSON unmarshals raw JSON bytes into the target struct.
func ParseJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
