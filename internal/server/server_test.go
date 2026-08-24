package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
)

const testKid = "test-key-1"

type sentMail struct {
	To, Title, Body string
}

type fakeSender struct {
	mu   sync.Mutex
	sent []sentMail
	err  error
}

func (f *fakeSender) Send(_ context.Context, to, title, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentMail{To: to, Title: title, Body: body})
	return nil
}

func (f *fakeSender) mails() []sentMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sentMail, len(f.sent))
	copy(out, f.sent)
	return out
}

type testIssuer struct {
	priv   *rsa.PrivateKey
	jwks   *httptest.Server
	issuer string
}

func newTestIssuer(t *testing.T, issuer string) *testIssuer {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ti := &testIssuer{priv: priv, issuer: issuer}
	ti.jwks = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		key := jose.JSONWebKey{
			Key:       &priv.PublicKey,
			KeyID:     testKid,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
	}))
	t.Cleanup(ti.jwks.Close)
	return ti
}

func (ti *testIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	now := time.Now()
	if _, ok := claims["iss"]; !ok {
		claims["iss"] = ti.issuer
	}
	if _, ok := claims["sub"]; !ok {
		claims["sub"] = "user-1"
	}
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = now.Add(time.Hour).Unix()
	}
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = now.Unix()
	}
	if _, ok := claims["azp"]; !ok {
		claims["azp"] = DefaultAllowedAZP
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", testKid)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: ti.priv}, opts)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := obj.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newTestServer(t *testing.T, sender *fakeSender) (*Server, *testIssuer) {
	t.Helper()
	ti := newTestIssuer(t, DefaultIssuer)
	ctx := oidc.ClientContext(context.Background(), ti.jwks.Client())
	srv := &Server{
		allowedAZP: DefaultAllowedAZP,
		verifier:   newVerifier(ctx, DefaultIssuer, ti.jwks.URL),
		sender:     sender,
		dedupe:     NewWindow(30 * time.Minute),
	}
	return srv, ti
}

func doAlert(t *testing.T, h http.Handler, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/alerts", rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, &fakeSender{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAlertsMissingToken(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, &fakeSender{})
	rec := doAlert(t, srv.Handler(), "", map[string]string{
		"title": "t", "body": "b", "dedupeKey": "k",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAlertsBadToken(t *testing.T) {
	t.Parallel()
	srv, _ := newTestServer(t, &fakeSender{})
	rec := doAlert(t, srv.Handler(), "not-a-jwt", map[string]string{
		"title": "t", "body": "b", "dedupeKey": "k",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAlertsWrongIssuer(t *testing.T) {
	t.Parallel()
	sender := &fakeSender{}
	srv, ti := newTestServer(t, sender)
	tok := ti.token(t, map[string]any{
		"iss":   "https://evil.example/realms/soundadam",
		"email": "a@soundadam.com",
	})
	rec := doAlert(t, srv.Handler(), tok, map[string]string{
		"title": "t", "body": "b", "dedupeKey": "k",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if len(sender.mails()) != 0 {
		t.Fatal("must not send")
	}
}

func TestAlertsWrongAZP(t *testing.T) {
	t.Parallel()
	sender := &fakeSender{}
	srv, ti := newTestServer(t, sender)
	tok := ti.token(t, map[string]any{
		"azp":   "other-client",
		"email": "a@soundadam.com",
	})
	rec := doAlert(t, srv.Handler(), tok, map[string]string{
		"title": "t", "body": "b", "dedupeKey": "k",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAlertsMissingEmail(t *testing.T) {
	t.Parallel()
	srv, ti := newTestServer(t, &fakeSender{})
	tok := ti.token(t, map[string]any{})
	rec := doAlert(t, srv.Handler(), tok, map[string]string{
		"title": "t", "body": "b", "dedupeKey": "k",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAlertsSendUsesJWTEmailNotBodyTo(t *testing.T) {
	t.Parallel()
	sender := &fakeSender{}
	srv, ti := newTestServer(t, sender)
	tok := ti.token(t, map[string]any{"email": "jwt@soundadam.com"})
	rec := doAlert(t, srv.Handler(), tok, map[string]string{
		"title":     "余座",
		"body":      "有空位",
		"dedupeKey": "sub:class-1",
		"to":        "attacker@example.com",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	mails := sender.mails()
	if len(mails) != 1 {
		t.Fatalf("sent %d", len(mails))
	}
	if mails[0].To != "jwt@soundadam.com" {
		t.Fatalf("to %q", mails[0].To)
	}
	if mails[0].Title != "余座" || mails[0].Body != "有空位" {
		t.Fatalf("mail %+v", mails[0])
	}
}

func TestAlertsDedupeStill200(t *testing.T) {
	t.Parallel()
	sender := &fakeSender{}
	srv, ti := newTestServer(t, sender)
	tok := ti.token(t, map[string]any{"email": "jwt@soundadam.com"})
	body := map[string]string{"title": "余座", "body": "有空位", "dedupeKey": "same-key"}
	h := srv.Handler()
	first := doAlert(t, h, tok, body)
	second := doAlert(t, h, tok, body)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses %d %d", first.Code, second.Code)
	}
	if len(sender.mails()) != 1 {
		t.Fatalf("want 1 send, got %d", len(sender.mails()))
	}
	var payload map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["deduped"] != true {
		t.Fatalf("second response %s", second.Body.String())
	}
}

func TestAlertsMissingFields(t *testing.T) {
	t.Parallel()
	srv, ti := newTestServer(t, &fakeSender{})
	tok := ti.token(t, map[string]any{"email": "jwt@soundadam.com"})
	rec := doAlert(t, srv.Handler(), tok, map[string]string{"title": "t"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
