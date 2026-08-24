package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/resend/resend-go/v3"
)

func TestResendSenderUsesFakeAPI(t *testing.T) {
	t.Parallel()
	var got resend.SendEmailRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" {
			t.Errorf("path %s", r.URL.Path)
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer re_test" {
			t.Errorf("auth %q", gotAuth)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fake-1"}`))
	}))
	t.Cleanup(ts.Close)

	client := resend.NewCustomClient(ts.Client(), "re_test")
	u, err := url.Parse(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	client.BaseURL = u
	sender := &ResendSender{client: client, from: DefaultMailFrom}
	if err := sender.Send(context.Background(), "jwt@soundadam.com", "余座", "有空位"); err != nil {
		t.Fatal(err)
	}
	if len(got.To) != 1 || got.To[0] != "jwt@soundadam.com" {
		t.Fatalf("to %#v", got.To)
	}
	if got.From != DefaultMailFrom || got.Subject != "余座" || got.Text != "有空位" {
		t.Fatalf("payload %+v", got)
	}
}

func TestAlertsSendFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	sender := &fakeSender{err: errSend}
	srv, ti := newTestServer(t, sender)
	tok := ti.token(t, map[string]any{"email": "jwt@soundadam.com"})
	body := map[string]string{"title": "t", "body": "b", "dedupeKey": "retry-key"}
	h := srv.Handler()
	first := doAlert(t, h, tok, body)
	if first.Code != http.StatusBadGateway {
		t.Fatalf("status %d body %s", first.Code, first.Body.String())
	}
	sender.mu.Lock()
	sender.err = nil
	sender.mu.Unlock()
	second := doAlert(t, h, tok, body)
	if second.Code != http.StatusOK {
		t.Fatalf("retry status %d body %s", second.Code, second.Body.String())
	}
	if n := len(sender.mails()); n != 1 {
		t.Fatalf("sends %d", n)
	}
}

type sendError string

func (e sendError) Error() string { return string(e) }

const errSend sendError = "resend down"
