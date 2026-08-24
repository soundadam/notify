package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

const maxAlertBody = 1 << 20

// Server is the notify HTTP API.
type Server struct {
	allowedAZP string
	verifier   *oidc.IDTokenVerifier
	sender     Sender
	dedupe     *Window
}

// New builds a production server (remote JWKS + Resend).
func New(cfg Config, httpClient *http.Client) (*Server, error) {
	if httpClient == nil {
		httpClient = ProxyHTTPClient(0)
	}
	ctx := oidc.ClientContext(context.Background(), httpClient)
	return &Server{
		allowedAZP: cfg.AllowedAZP,
		verifier:   newVerifier(ctx, cfg.Issuer, cfg.JWKSURL),
		sender:     NewResendSender(cfg.ResendAPIKey, cfg.MailFrom, httpClient),
		dedupe:     NewWindow(cfg.DedupeTTL),
	}, nil
}

type alertRequest struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	DedupeKey string `json:"dedupeKey"`
	To        string `json:"to"` // ignored; recipient is JWT email only
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/alerts", s.handleAlerts)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	raw, err := bearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "缺少或无效的令牌")
		return
	}
	id, err := s.verifyAccessToken(r.Context(), raw)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "缺少或无效的令牌")
		return
	}
	if id.Email == "" {
		writeError(w, http.StatusForbidden, "令牌没有 email")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAlertBody)
	defer r.Body.Close()
	var req alertRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.DedupeKey = strings.TrimSpace(req.DedupeKey)
	if req.Title == "" || req.Body == "" || req.DedupeKey == "" {
		writeError(w, http.StatusBadRequest, "title、body、dedupeKey 均为必填")
		return
	}

	if s.dedupe.Reserve(req.DedupeKey) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deduped": true})
		return
	}

	if err := s.sender.Send(r.Context(), id.Email, req.Title, req.Body); err != nil {
		s.dedupe.Forget(req.DedupeKey)
		log.Printf("发送失败 sub=%s key=%s: %v", id.Subject, req.DedupeKey, err)
		writeError(w, http.StatusBadGateway, "发送失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
