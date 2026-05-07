// Package utils contains small helper functions shared across the whole project.
// jwt.go — generating and validating JWT tokens.
package utils

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the data we store inside the JWT token.
// When we parse a token we get this struct back, giving us the user's ID and device ID.
type Claims struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId"`
	jwt.RegisteredClaims // standard fields: expiry, issued-at, issuer, etc.
}

// jwtSecret reads the signing secret from the environment.
// Never hardcode this in production — set JWT_SECRET in your .env file.
func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "fallback-secret-CHANGE-ME-in-production"
	}
	return []byte(s)
}

// tokenExpiry reads JWT_EXPIRY_HOURS from env (default: 720 hours = 30 days).
func tokenExpiry() time.Duration {
	hours, err := strconv.Atoi(os.Getenv("JWT_EXPIRY_HOURS"))
	if err != nil || hours <= 0 {
		hours = 720
	}
	return time.Duration(hours) * time.Hour
}

// GenerateToken creates a signed JWT for the given user and device.
// Returns the token string, its expiry time, and any error.
func GenerateToken(userID, deviceID string) (string, time.Time, error) {
	expiry := time.Now().UTC().Add(tokenExpiry())

	claims := &Claims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			Issuer:    "ranchat",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret())
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiry, nil
}

// ParseToken validates a token string and returns the claims inside it.
// Returns an error if the token is invalid, expired, or tampered with.
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Make sure the signing algorithm is what we expect
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
