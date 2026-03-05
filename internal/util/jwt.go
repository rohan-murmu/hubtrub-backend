package util

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the claims in the JWT token
type TokenClaims struct {
	ClientID       string `json:"clientId"`
	ClientUserName string `json:"clientUserName"`
	ClientAvatar   string `json:"clientAvatar"`
	jwt.RegisteredClaims
}

const (
	// SecretKey is used to sign and verify tokens (in production, load from env)
	SecretKey = "hubtrub-secret-key-change-in-production"
	// TokenExpiry is the duration for which a token is valid
	TokenExpiry = 2 * 24 * time.Hour // 2 days
)

// GenerateToken creates a new JWT token for a client
func GenerateToken(clientID, clientUserName string, clientAvatar string) (string, error) {
	claims := TokenClaims{
		ClientID:       clientID,
		ClientUserName: clientUserName,
		ClientAvatar:   clientAvatar,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*TokenClaims, error) {
	claims := &TokenClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
