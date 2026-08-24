package server

import (
	"testing"
	"time"
)

func TestNormalizeJWKSURL(t *testing.T) {
	t.Parallel()
	issuer := DefaultIssuer
	tests := []struct {
		name, override, want string
	}{
		{
			name:     "default public issuer",
			override: "",
			want:     issuer + "/protocol/openid-connect/certs",
		},
		{
			name:     "in-cluster realm base",
			override: "http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam",
			want:     "http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam/protocol/openid-connect/certs",
		},
		{
			name:     "full certs URL",
			override: "http://keycloak.example/realms/soundadam/protocol/openid-connect/certs",
			want:     "http://keycloak.example/realms/soundadam/protocol/openid-connect/certs",
		},
		{
			name:     "generic jwks path",
			override: "https://auth.example/jwks",
			want:     "https://auth.example/jwks",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeJWKSURL(issuer, tc.override)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestConfigFromEnvRequiresAPIKey(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_JWKS_URL", "http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam")
	t.Setenv("DEDUPE_TTL", "")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("MAIL_FROM", "")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issuer != DefaultIssuer || cfg.AllowedAZP != DefaultAllowedAZP {
		t.Fatalf("%+v", cfg)
	}
	if cfg.JWKSURL != "http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam/protocol/openid-connect/certs" {
		t.Fatalf("jwks %s", cfg.JWKSURL)
	}
	if cfg.DedupeTTL != 30*time.Minute || cfg.ListenAddr != ":8080" {
		t.Fatalf("%+v", cfg)
	}
}
