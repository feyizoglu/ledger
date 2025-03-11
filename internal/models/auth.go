package models

import "github.com/golang-jwt/jwt/v5"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	Role   string `json:"role"`
	UserID int64  `json:"user_id"`
}

// Claims defines the JWT claims structure
type Claims struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
