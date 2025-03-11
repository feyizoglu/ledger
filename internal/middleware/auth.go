package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ledger/internal/models"
	"ledger/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

const secretKey = "your-secret-key-change-in-production" // In production, use environment variable

// GenerateToken creates a new JWT token for a user
func GenerateToken(userID int64, role string) (string, error) {
	claims := models.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// AuthMiddleware returns a chi-compatible middleware
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
			http.Error(w, "Invalid token format", http.StatusUnauthorized)
			return
		}

		tokenString := bearerToken[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// Add claims to request context
			r.Header.Set("user_id", fmt.Sprintf("%v", claims["user_id"]))
			r.Header.Set("user_role", fmt.Sprintf("%v", claims["role"]))
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}
	})
}

// AdminOnly middleware restricts access to admin users
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("user").(*models.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if claims.Role != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// OwnerOrAdmin middleware allows access to resource owner or admin
func OwnerOrAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("user").(*models.Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Admins can access everything
		if claims.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		// For non-admins, check if they're accessing their own data
		userID, err := utils.GetUserIDFromPath(r)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		if userID != claims.UserID {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
