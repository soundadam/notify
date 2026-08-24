package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/resend/resend-go/v3"
)

// Sender delivers one alert email. Tests inject a fake; production uses Resend.
type Sender interface {
	Send(ctx context.Context, to, title, body string) error
}

// ResendSender sends via resend-go. The HTTP client should honor HTTP_PROXY.
type ResendSender struct {
	client *resend.Client
	from   string
}

func NewResendSender(apiKey, from string, httpClient *http.Client) *ResendSender {
	if httpClient == nil {
		httpClient = ProxyHTTPClient(time.Minute)
	}
	return &ResendSender{
		client: resend.NewCustomClient(httpClient, apiKey),
		from:   from,
	}
}

func (s *ResendSender) Send(ctx context.Context, to, title, body string) error {
	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: title,
		Text:    body,
	}
	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	return nil
}
