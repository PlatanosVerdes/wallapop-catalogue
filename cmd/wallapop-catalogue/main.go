package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/PlatanosVerdes/wallapop-catalogue/internal/commands"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/config"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/metrics"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/reactivate"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/server"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/session"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/telegram"
	"github.com/PlatanosVerdes/wallapop-catalogue/internal/wallapop"
)

var buildVersion = "local"

const usage = `wallapop-catalogue ` + "%s" + `

  run [--dry-run]        one pass: reactivate everything expired
  serve [--port] [--interval]
                         the daily pass, the services bot's commands and /healthz
  session import --cookie <value>
                         store the browser session cookie (also reads it on stdin)
  session show           what is stored and whether it can renew itself
  session refresh        renew now, which is how you check the session works
  sign <method> <path> <timestamp> [signature]
                         reproduce an X-Signature, or say which scheme matches a captured one

Configured through the environment (WALLA_*); see the README.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Printf(usage, buildVersion)
		return errors.New("no command given")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	log := newLogger(cfg.LogJSON)
	store := session.NewStore(cfg.DataDir)

	switch args[0] {
	case "run":
		return cmdRun(cfg, store, log, args[1:])
	case "serve":
		return cmdServe(cfg, store, log, args[1:])
	case "session":
		return cmdSession(cfg, store, args[1:])
	case "sign":
		return cmdSign(cfg, args[1:])
	case "help", "-h", "--help":
		fmt.Printf(usage, buildVersion)
		return nil
	default:
		fmt.Printf(usage, buildVersion)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func newLogger(asJSON bool) *slog.Logger {
	if asJSON {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func newClient(cfg config.Config, store *session.Store) *wallapop.Client {
	client := wallapop.New(store)
	client.BaseURL = cfg.BaseURL
	client.WebURL = cfg.WebURL
	client.Scheme = cfg.Scheme
	client.AppVersion = cfg.AppVersion
	client.DeviceID = cfg.DeviceID
	if client.DeviceID == "" {
		if sess := store.Current(); sess != nil {
			client.DeviceID = sess.Device()
		}
	}
	return client
}

func cmdRun(cfg config.Config, store *session.Store, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "list what would be reactivated without touching anything")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	res := onePass(ctx, cfg, store, log, *dryRun)
	fmt.Println(res.Summary())
	if !res.OK() {
		return errors.New("the pass did not finish clean")
	}
	return nil
}

func cmdServe(cfg config.Config, store *session.Store, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", cfg.Port, "port for /healthz")
	interval := fs.Duration("interval", cfg.Interval, "time between reactivation passes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var next atomic.Int64
	next.Store(time.Now().Unix())
	nextRun := func() time.Time { return time.Unix(next.Load(), 0) }
	health := &server.Health{
		Version:    buildVersion,
		DataDir:    cfg.DataDir,
		Store:      store,
		WarnBefore: cfg.WarnBefore,
		NextRun:    nextRun,
	}
	mux := http.NewServeMux()
	mux.Handle("/", health.Handler())
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(*port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Info("listening", "addr", srv.Addr, "version", buildVersion)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "err", err)
			stop()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			res := scheduledPass(ctx, cfg, store, log)

			// Retry soon, so a session imported by hand works in minutes, not tomorrow.
			wait := *interval
			if !res.OK() {
				wait = cfg.RetryEvery
			}
			at := time.Now().Add(wait)
			next.Store(at.Unix())
			log.Info("next pass", "at", at.Format(time.RFC3339), "after_failure", !res.OK())
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()

	if bot := telegram.New(cfg.TelegramToken, cfg.TelegramChat); bot.Enabled() {
		b := &botState{cfg: cfg, store: store, log: log, nextRun: nextRun}
		listener := &commands.Listener{
			Bot:     bot,
			Allowed: func(chat string) bool { return chat == cfg.TelegramChat },
			Log:     log,
		}
		listener.Commands = b.commands(listener)
		if cfg.TelegramPush {
			pushUpdates(ctx, mux, listener, log)
		} else {
			go func() {
				if err := listener.Serve(ctx); err != nil {
					log.Error("the bot stopped listening", "err", err)
				}
			}()
		}
	}

	wg.Wait()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

// Two passes at once would reactivate the same listing twice.
var passing sync.Mutex

func scheduledPass(ctx context.Context, cfg config.Config, store *session.Store, log *slog.Logger) reactivate.Result {
	passing.Lock()
	defer passing.Unlock()
	return onePass(ctx, cfg, store, log, false)
}

// onePass never sends a message itself: the alert rules decide.
func onePass(ctx context.Context, cfg config.Config, store *session.Store, log *slog.Logger, dryRun bool) reactivate.Result {
	report := func(res reactivate.Result) reactivate.Result {
		if err := reactivate.SaveResult(cfg.DataDir, res); err != nil {
			log.Error("could not save the run", "err", err)
		}
		if dryRun {
			return res
		}
		if err := push(ctx, cfg, store, res); err != nil {
			log.Error("could not push metrics", "err", err)
		}
		return res
	}

	if _, err := store.Load(); err != nil {
		log.Error("no usable session", "err", err)
		return report(reactivate.Result{StartedAt: time.Now(), Error: err.Error(), NeedsHuman: true})
	}
	client := newClient(cfg, store)

	// The access token lasts five minutes, so it is always spent between passes.
	if store.AccessSpent() {
		if err := client.RenewSession(ctx); err != nil {
			log.Error("the session could not be renewed", "err", err)
			return report(reactivate.Result{StartedAt: time.Now(), Error: err.Error(), NeedsHuman: true})
		}
		log.Info("session renewed")
	}

	return report(reactivate.Run(ctx, client, reactivate.Options{
		DryRun:    dryRun,
		MinPause:  cfg.MinPause,
		MaxPause:  cfg.MaxPause,
		MaxPerRun: cfg.MaxPerRun,
	}, log))
}

// Status: 0 fine, 1 a retry may fix it, 2 needs a human.
func push(ctx context.Context, cfg config.Config, store *session.Store, res reactivate.Result) error {
	status := 0.0
	switch {
	case res.NeedsHuman:
		status = 2
	case !res.OK():
		status = 1
	}

	gauges := []metrics.Gauge{
		{Name: "wallapop_last_run_status", Help: "0 fine, 1 failed, 2 needs a human", Value: status},
		{Name: "wallapop_last_run_timestamp", Help: "Unix time of the last pass", Value: float64(time.Now().Unix())},
		{Name: "wallapop_expired_listings", Help: "Listings found expired in the last pass", Value: float64(res.Expired)},
		{Name: "wallapop_reactivated_listings", Help: "Listings reactivated in the last pass", Value: float64(len(res.Reactivated))},
	}
	// -1 when not known yet.
	days := -1.0
	if sess := store.Current(); sess != nil {
		if left, ok := sess.Renewable(); ok {
			days = left.Hours() / 24
		}
	}
	gauges = append(gauges, metrics.Gauge{
		Name:  "wallapop_session_days_remaining",
		Help:  "Days before the session has to be imported again by hand",
		Value: days,
	})

	return metrics.New(cfg.Pushgateway, "wallapop-catalogue").Push(ctx, gauges)
}

func cmdSession(cfg config.Config, store *session.Store, args []string) error {
	if len(args) == 0 {
		return errors.New("session needs a subcommand: import, show or refresh")
	}

	switch args[0] {
	case "import":
		fs := flag.NewFlagSet("session import", flag.ExitOnError)
		cookie := fs.String("cookie", "", "the "+session.DefaultCookieName+" cookie from the browser")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		var sess *session.Session
		var err error
		if *cookie != "" {
			sess, err = session.Parse(*cookie)
		} else {
			var raw []byte
			if raw, err = io.ReadAll(os.Stdin); err == nil {
				sess, err = session.Parse(string(raw))
			}
		}
		if err != nil {
			return err
		}
		if err := store.Save(sess); err != nil {
			return err
		}
		fmt.Printf("session stored in %s\n", store.Path())

		// Renew now to prove the imported cookie works.
		if err := newClient(cfg, store).RenewSession(context.Background()); err != nil {
			return fmt.Errorf("stored, but it cannot mint a token: %w", err)
		}
		fmt.Println("renewed once, so the session works")
		printSession(store.Current())
		return nil

	case "show":
		sess, err := store.Load()
		if err != nil {
			return err
		}
		printSession(sess)
		return nil

	case "refresh":
		if _, err := store.Load(); err != nil {
			return err
		}
		if err := newClient(cfg, store).RenewSession(context.Background()); err != nil {
			return err
		}
		fmt.Println("renewed")
		printSession(store.Current())
		return nil

	default:
		return fmt.Errorf("unknown session subcommand %q", args[0])
	}
}

func printSession(sess *session.Session) {
	fmt.Printf("cookie:    %s\n", sess.CookieName)
	if claims, ok := sess.Claims(); ok {
		fmt.Printf("user:      %s\n", claims.Sub)
		fmt.Printf("device:    %s\n", sess.Device())
		fmt.Printf("token:     %s left\n", time.Until(time.Unix(claims.Exp, 0)).Round(time.Second))
	} else {
		fmt.Println("token:     none minted yet")
	}
	if left, ok := sess.Renewable(); ok {
		fmt.Printf("renewable: %s left (until %s)\n", left.Round(time.Hour), sess.Expires.Format(time.RFC3339))
	} else {
		fmt.Println("renewable: unknown until the first renewal")
	}
}

// cmdSign is kept for when Wallapop brings request signing back.
func cmdSign(cfg config.Config, args []string) error {
	if len(args) < 3 {
		return errors.New("sign needs: <method> <path> <timestamp> [signature]")
	}
	method, path := args[0], args[1]
	ts, err := strconv.ParseInt(strings.TrimSpace(args[2]), 10, 64)
	if err != nil {
		return fmt.Errorf("timestamp: %w", err)
	}

	if len(args) >= 4 {
		if scheme, ok := wallapop.MatchScheme(method, path, ts, args[3]); ok {
			fmt.Printf("matches scheme %q, set WALLA_SIGN_SCHEME=%s\n", scheme, scheme)
			return nil
		}
		fmt.Println("no known scheme reproduces that signature")
		for _, scheme := range wallapop.Schemes() {
			got, err := wallapop.Sign(scheme, method, path, ts)
			if err != nil {
				return err
			}
			fmt.Printf("  %-7s %s\n", scheme, got)
		}
		return errors.New("signature not reproduced")
	}

	scheme := cfg.Scheme
	if scheme == wallapop.SchemeNone {
		scheme = wallapop.SchemePipe
	}
	sig, err := wallapop.Sign(scheme, method, path, ts)
	if err != nil {
		return err
	}
	fmt.Println(sig)
	return nil
}
