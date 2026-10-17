package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnabled(t *testing.T) {
	if New("", "").Enabled() {
		t.Error("a bot with no token reported itself ready")
	}
	if New("token", "").Enabled() {
		t.Error("a bot with no chat reported itself ready")
	}
	if !New("token", "chat").Enabled() {
		t.Error("a configured bot reported itself unusable")
	}
}

func TestNothingIsSentWithoutAToken(t *testing.T) {
	if err := New("", "").Text(context.Background(), "y", nil); err != nil {
		t.Errorf("an unconfigured bot returned an error instead of staying quiet: %v", err)
	}
}

func TestEscape(t *testing.T) {
	if got := Escape(`Kallax <b>barata</b> & buena`); got != "Kallax &lt;b&gt;barata&lt;/b&gt; &amp; buena" {
		t.Errorf("escaped to %q", got)
	}
}

func TestErrorsDoNotCarryTheToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	bot := New("secret-token", "chat")
	bot.APIURL = srv.URL + "/bot"
	err := bot.Text(context.Background(), "hola", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("the error carries the token: %v", err)
	}
}

func TestTransportErrorsDoNotCarryTheToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	bot := New("secret-token", "chat")
	bot.APIURL = srv.URL + "/bot"
	for name, err := range map[string]error{
		"sendMessage": bot.Text(context.Background(), "hola", nil),
		"getUpdates":  func() error { _, err := bot.Updates(context.Background(), 0); return err }(),
	} {
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if strings.Contains(err.Error(), "secret-token") {
			t.Fatalf("%s: the error carries the token: %v", name, err)
		}
	}
}
