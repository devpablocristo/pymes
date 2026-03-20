package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type JWKSVerifier struct {
	keyfunc keyfunc.Keyfunc
}

func NewJWKSVerifier(jwksURL string) (*JWKSVerifier, error) {
	if strings.TrimSpace(jwksURL) == "" {
		return nil, errors.New("jwks url is required")
	}
	kf, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("create keyfunc: %w", err)
	}
	return &JWKSVerifier{keyfunc: kf}, nil
}

func (v *JWKSVerifier) VerifyToken(ctx context.Context, token string) (*jwt.Token, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("token is required")
	}
	parsed, err := jwt.Parse(token, v.keyfunc.KeyfuncCtx(ctx))
	if err != nil {
		return nil, fmt.Errorf("parse and verify token: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return parsed, nil
}
