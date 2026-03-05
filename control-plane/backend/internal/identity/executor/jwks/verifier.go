package jwks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Verifier valida tokens JWT usando JWKS remoto con cache interna.
type Verifier struct {
	keyfunc keyfunc.Keyfunc
}

func NewVerifier(jwksURL string) (*Verifier, error) {
	if strings.TrimSpace(jwksURL) == "" {
		return nil, errors.New("jwks url is required")
	}
	kf, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("create keyfunc: %w", err)
	}
	return &Verifier{keyfunc: kf}, nil
}

func (v *Verifier) VerifyToken(ctx context.Context, token string) (*jwt.Token, error) {
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
