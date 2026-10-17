package main

import (
	"strings"
	"testing"
	"time"

	"github.com/PlatanosVerdes/wallapop-catalogue/internal/reactivate"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/session"
)

func TestStatusOfAQuietPass(t *testing.T) {
	now := time.Date(2026, 10, 18, 10, 0, 0, 0, time.Local)
	last := reactivate.Result{StartedAt: now.Add(-11 * time.Hour), Catalogue: 6}
	sess := session.New("cookie")
	sess.Expires = now.Add(27*24*time.Hour + time.Hour)

	got := status(last, true, now.Add(13*time.Hour), sess, now)
	for _, want := range []string{"ayer 23:00", "6 anuncios, ninguno caducado", "Próxima: hoy 23:00", "renovable 27 días más"} {
		if !strings.Contains(got, want) {
			t.Errorf("status lacks %q:\n%s", want, got)
		}
	}
}

func TestStatusSaysTheSessionIsGone(t *testing.T) {
	now := time.Date(2026, 10, 18, 10, 0, 0, 0, time.Local)
	last := reactivate.Result{StartedAt: now, Error: "wallapop: session rejected", NeedsHuman: true}

	got := status(last, true, time.Time{}, nil, now)
	for _, want := range []string{"⚠️ wallapop: session rejected", "no hay ninguna, hay que importarla"} {
		if !strings.Contains(got, want) {
			t.Errorf("status lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Próxima") {
		t.Errorf("a pass with no time was announced:\n%s", got)
	}
}

func TestOutcomeCountsFailures(t *testing.T) {
	res := reactivate.Result{Expired: 3, Reactivated: []string{"a", "b"}, Failures: []reactivate.Failure{{ID: "c"}}}
	if got := outcome(res); got != "2 de 3 caducados reactivados, ⚠️ 1 fallidos" {
		t.Errorf("outcome reads %q", got)
	}
}
