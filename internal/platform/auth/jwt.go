package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reggieanim/jot/internal/modules/users/domain"
)

const tokenExpiry = 7 * 24 * time.Hour // 7 days

type JWTIssuer struct {
	secret []byte
}

func NewJWTIssuer(secret string) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret)}
}

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (j *JWTIssuer) Issue(userID domain.UserID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: string(userID),
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenExpiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Parse validates and returns the claims from a JWT string.
func (j *JWTIssuer) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
