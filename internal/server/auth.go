package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type identity struct {
	Subject string
	Email   string
	AZP     string
}

type tokenClaims struct {
	Email string `json:"email"`
	AZP   string `json:"azp"`
}

func newVerifier(ctx context.Context, issuer, jwksURL string) *oidc.IDTokenVerifier {
	keySet := oidc.NewRemoteKeySet(ctx, jwksURL)
	return oidc.NewVerifier(issuer, keySet, &oidc.Config{
		SkipClientIDCheck:    true,
		SupportedSigningAlgs: []string{"RS256", "PS256", "ES256"},
	})
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("缺少 Authorization")
	}
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", errors.New("Authorization 必须是 Bearer")
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", errors.New("缺少访问令牌")
	}
	return token, nil
}

func (s *Server) verifyAccessToken(ctx context.Context, raw string) (identity, error) {
	tok, err := s.verifier.Verify(ctx, raw)
	if err != nil {
		return identity{}, fmt.Errorf("校验失败: %w", err)
	}
	var claims tokenClaims
	if err := tok.Claims(&claims); err != nil {
		return identity{}, fmt.Errorf("读取 claims 失败: %w", err)
	}
	if claims.AZP != s.allowedAZP {
		return identity{}, fmt.Errorf("azp 不允许: %q", claims.AZP)
	}
	return identity{
		Subject: tok.Subject,
		Email:   strings.TrimSpace(claims.Email),
		AZP:     claims.AZP,
	}, nil
}
