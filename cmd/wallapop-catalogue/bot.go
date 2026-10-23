package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PlatanosVerdes/wallapop-catalogue/internal/commands"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/config"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/reactivate"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/session"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/telegram"
)

// The services bot is shared, so every command carries the service's prefix.
const prefix = "wallapop_"

var errPassRunning = errors.New("ya hay una pasada en marcha")

type botState struct {
	cfg     config.Config
	store   *session.Store
	log     *slog.Logger
	bot     *telegram.Bot
	nextRun func() time.Time
}

func (b *botState) commands(listener *commands.Listener) []commands.Command {
	b.bot = listener.Bot
	return []commands.Command{
		{
			Name: prefix + "estado",
			Help: "Catálogo de Wallapop y sesión",
			Run: func(context.Context, commands.Request) (commands.Reply, error) {
				last, ok := reactivate.LoadResult(b.cfg.DataDir)
				return commands.Say(status(last, ok, b.nextRun(), b.store.Current(), time.Now())), nil
			},
		},
		{
			Name: prefix + "reactivar",
			Help: "Reactivar lo caducado ahora",
			Run:  b.passNow,
		},
	}
}

// passNow answers at once: with the pauses between listings a pass can take half an hour.
func (b *botState) passNow(ctx context.Context, req commands.Request) (commands.Reply, error) {
	if !passing.TryLock() {
		return commands.Reply{}, errPassRunning
	}
	go func() {
		defer passing.Unlock()
		res := onePass(ctx, b.cfg, b.store, b.log, false)
		if err := b.bot.To(req.ChatID()).Text(ctx, telegram.Escape(res.Summary()), nil); err != nil {
			b.log.Error("could not send the pass", "err", err)
		}
	}()
	return commands.Say("Empiezo la pasada, te aviso al acabar."), nil
}

func status(last reactivate.Result, ok bool, next time.Time, sess *session.Session, now time.Time) string {
	var b strings.Builder
	b.WriteString("<b>Catálogo de Wallapop</b>\n")
	switch {
	case !ok:
		b.WriteString("Última pasada: ninguna todavía\n")
	case last.Error != "":
		fmt.Fprintf(&b, "Última pasada: %s - ⚠️ %s\n", when(last.StartedAt, now), telegram.Escape(last.Error))
	default:
		fmt.Fprintf(&b, "Última pasada: %s - %s\n", when(last.StartedAt, now), outcome(last))
	}
	if !next.IsZero() && next.After(now) {
		fmt.Fprintf(&b, "Próxima: %s\n", when(next, now))
	}
	b.WriteString("Sesión: " + sessionLine(sess, now))
	return b.String()
}

func outcome(res reactivate.Result) string {
	if res.Expired == 0 {
		return fmt.Sprintf("%d anuncios, ninguno caducado", res.Catalogue)
	}
	line := fmt.Sprintf("%d de %d caducados reactivados", len(res.Reactivated), res.Expired)
	if len(res.Failures) > 0 {
		line += fmt.Sprintf(", ⚠️ %d fallidos", len(res.Failures))
	}
	return line
}

func sessionLine(sess *session.Session, now time.Time) string {
	if sess == nil || sess.CookieValue == "" {
		return "⚠️ no hay ninguna, hay que importarla"
	}
	if sess.Expires.IsZero() {
		return "importada, sin renovar todavía"
	}
	left := sess.Expires.Sub(now)
	if left <= 0 {
		return "⚠️ caducada, hay que importarla otra vez"
	}
	days := int(left.Hours() / 24)
	if days < 1 {
		return "⚠️ caduca hoy"
	}
	return fmt.Sprintf("renovable %d días más", days)
}

func when(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	day := t.Format("2006-01-02")
	switch day {
	case now.Format("2006-01-02"):
		return "hoy " + t.Format("15:04")
	case now.AddDate(0, 0, 1).Format("2006-01-02"):
		return "mañana " + t.Format("15:04")
	case now.AddDate(0, 0, -1).Format("2006-01-02"):
		return "ayer " + t.Format("15:04")
	}
	return t.Format("2/1 15:04")
}

// pushUpdates takes the bot's updates from switchboard instead of polling Telegram.
func pushUpdates(ctx context.Context, mux *http.ServeMux, listener *commands.Listener, log *slog.Logger) {
	mux.HandleFunc("POST /telegram/update", func(w http.ResponseWriter, r *http.Request) {
		var update telegram.Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "not an update", http.StatusBadRequest)
			return
		}
		// Answered at once: switchboard waits only 15 s and some commands take longer.
		go listener.Dispatch(ctx, update)
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /telegram/commands", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(listener.Menu()); err != nil {
			log.Error("could not send the commands", "err", err)
		}
	})
	log.Info("bot updates come from switchboard")
}
