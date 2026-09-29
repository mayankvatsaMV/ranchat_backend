package utils

import (
	"fmt"
	"log"
	"os"
	"ranchat/internals/authentication/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	jwt.RegisteredClaims
}

var secretKey []byte

// InitJWTSecret initializes and caches the JWT secret from config
func InitJWTSecret(secret string) {
	secretKey = []byte(secret)
}

func getJWTSecret() []byte {
	if len(secretKey) == 0 {
		secretKey = []byte(os.Getenv("JWT_SECRET"))
	}
	return secretKey
}

func GenerateJWTToken(user models.User) string {

	claims := Claims{
		ID:   user.UserId.Hex(),
		Name: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(
				time.Now().Add(100 * 365 * 24 * time.Hour),
			),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(getJWTSecret())
	if err != nil {
		log.Fatalf("Error signing token: %v", err)
	}
	return tokenString
}

func VerifyJWTToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	// Parse and validate the token
	parsedToken, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return getJWTSecret(), nil
	})

	if err != nil {
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid or expired token")
	}

	return claims, nil
}
