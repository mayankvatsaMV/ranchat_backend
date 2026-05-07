// time.go — helpers for formatting Go time.Time values as strings for the API.
package utils

import "time"

// FormatTime converts a time.Time to an ISO-8601 string (e.g. "2025-01-01T10:00:00Z").
// All times are in UTC so clients in any timezone get a consistent format.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// FormatTimePtr does the same for a *time.Time pointer.
// Returns nil if the pointer itself is nil (e.g. for premiumTill on free users).
func FormatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := FormatTime(*t)
	return &s
}
