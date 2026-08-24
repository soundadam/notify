package server

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultIssuer     = "https://auth.soundadam.com/realms/soundadam"
	DefaultAllowedAZP = "soundadam-scli"
	DefaultListenAddr = ":8080"
	DefaultMailFrom   = "soundadam <no-reply@soundadam.com>"
	DefaultDedupeTTL  = 30 * time.Minute
	keycloakCertsPath = "/protocol/openid-connect/certs"
)

// Config is runtime configuration from the process environment.
type Config struct {
	ListenAddr   string
	Issuer       string
	JWKSURL      string
	AllowedAZP   string
	ResendAPIKey string
	MailFrom     string
	DedupeTTL    time.Duration
}

// ConfigFromEnv loads Config. RESEND_API_KEY is required in production.
func ConfigFromEnv() (Config, error) {
	ttl := DefaultDedupeTTL
	if raw := strings.TrimSpace(os.Getenv("DEDUPE_TTL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("DEDUPE_TTL: %w", err)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("DEDUPE_TTL 必须为正")
		}
		ttl = parsed
	}

	issuer := envOr("OIDC_ISSUER", DefaultIssuer)
	cfg := Config{
		ListenAddr:   envOr("LISTEN_ADDR", DefaultListenAddr),
		Issuer:       issuer,
		JWKSURL:      NormalizeJWKSURL(issuer, os.Getenv("OIDC_JWKS_URL")),
		AllowedAZP:   envOr("OIDC_ALLOWED_AZP", DefaultAllowedAZP),
		ResendAPIKey: strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		MailFrom:     envOr("MAIL_FROM", DefaultMailFrom),
		DedupeTTL:    ttl,
	}
	if cfg.ResendAPIKey == "" {
		return Config{}, fmt.Errorf("缺少 RESEND_API_KEY")
	}
	return cfg, nil
}

// NormalizeJWKSURL turns a Keycloak realm base (or a full JWKS URL) into a certs URL.
//
// In-cluster GitOps should set OIDC_JWKS_URL to
// http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam
func NormalizeJWKSURL(issuer, override string) string {
	base := strings.TrimRight(strings.TrimSpace(override), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(issuer), "/")
	}
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, "/protocol/openid-connect/certs") ||
		strings.Contains(lower, "/jwks") {
		return base
	}
	return base + keycloakCertsPath
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
