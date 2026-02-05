package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/scythrine05/hubtrub-server/internal/util"
)

// ContextKey is used for storing authenticated client info in context
type ContextKey string

const (
	// ClientIDKey stores the authenticated client ID in context
	ClientIDKey ContextKey = "clientID"
	// ClientUserNameKey stores the authenticated client username in context
	ClientUserNameKey ContextKey = "clientUserName"
)

// AuthMiddleware validates JWT tokens from Authorization header
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>" format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := util.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Store client info in context
		ctx := context.WithValue(r.Context(), ClientIDKey, claims.ClientID)
		ctx = context.WithValue(ctx, ClientUserNameKey, claims.ClientUserName)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClientIDFromContext retrieves the authenticated client ID from context
func GetClientIDFromContext(r *http.Request) (string, bool) {
	clientID := r.Context().Value(ClientIDKey)
	if clientID == nil {
		return "", false
	}
	return clientID.(string), true
}

// GetClientUserNameFromContext retrieves the authenticated client username from context
func GetClientUserNameFromContext(r *http.Request) (string, bool) {
	clientUserName := r.Context().Value(ClientUserNameKey)
	if clientUserName == nil {
		return "", false
	}
	return clientUserName.(string), true
}
