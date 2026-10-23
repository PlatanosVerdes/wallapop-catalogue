package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PlatanosVerdes/wallapop-catalogue/internal/commands"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/telegram"
)

func TestPushedUpdateRunsTheCommand(t *testing.T) {
	sent := make(chan string, 1)
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if strings.HasSuffix(r.URL.Path, "sendMessage") {
			sent <- r.Form.Get("text")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer tg.Close()
	bot := telegram.New("token", "42")
	bot.APIURL = tg.URL + "/bot"

	listener := &commands.Listener{
		Bot:     bot,
		Allowed: func(chat string) bool { return chat == "42" },
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Commands: []commands.Command{{
			Name: prefix + "estado",
			Help: "Estado",
			Run: func(context.Context, commands.Request) (commands.Reply, error) {
				return commands.Say("todo bien"), nil
			},
		}},
	}
	mux := http.NewServeMux()
	pushUpdates(context.Background(), mux, listener, listener.Log)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	update := `{"update_id":1,"message":{"text":"/wallapop_estado","chat":{"id":42,"type":"private"}}}`
	resp, err := http.Post(srv.URL+"/telegram/update", "application/json", strings.NewReader(update))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d, want 202", resp.StatusCode)
	}
	select {
	case got := <-sent:
		if got != "todo bien" {
			t.Fatalf("answered %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the command never answered")
	}

	resp, err = http.Get(srv.URL + "/telegram/commands")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var menu []telegram.Command
	if err := json.NewDecoder(resp.Body).Decode(&menu); err != nil {
		t.Fatal(err)
	}
	if len(menu) != 1 || menu[0].Name != "wallapop_estado" {
		t.Fatalf("menu %+v", menu)
	}
}

func TestPushRejectsWhatIsNotAnUpdate(t *testing.T) {
	mux := http.NewServeMux()
	listener := &commands.Listener{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	pushUpdates(context.Background(), mux, listener, listener.Log)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/telegram/update", "application/json", strings.NewReader("{"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
}
