# wallapop-catalogue

Reactivates my expired Wallapop listings once a day, and answers for it in
[@PlatanosServicesBot](https://t.me/PlatanosServicesBot).

Runs on the Raspberry from [rpi-services](https://github.com/PlatanosVerdes/rpi-services). Go, no
dependencies, CalVer tag on every push to main. The searches bot for friends is a separate service,
[wallabot](https://github.com/PlatanosVerdes/wallabot).

## The services bot

@PlatanosServicesBot is shared by the small services on the Pi, so every command carries the
service's prefix. Only the owner's chat (`WALLA_TELEGRAM_CHAT`) is answered.

| Command | What it does |
| :--- | :--- |
| `/wallapop_estado` | Last pass, next one, and how long the session can still renew itself |
| `/wallapop_reactivar` | A pass right now. It answers at once and reports when it ends |

Telegram hands each update to one reader only, so this service is the only one that may call
`getUpdates` with that token. Any service can still send to it.

Nothing is sent on its own: a dead or expiring session and a pass that stopped running are Grafana
alerts, built on the metrics below.

## The pass

Reads the catalogue with `GET /api/v3/user/items` and reactivates each expired listing with
`PUT /api/v3/items/{id}/reactivate`, once a day, with a random pause between listings. A failed pass
is retried after `WALLA_RETRY_EVERY`, so a session imported by hand works within minutes.

## Session

The session is the browser cookie `__Secure-next-auth.session-token`. The service renews the
5-minute access token from it on its own; the cookie lasts about a month. To import it, copy it from
DevTools (Application → Cookies → es.wallapop.com):

```bash
docker exec wallapop-catalogue wallapop-catalogue session import --cookie '<value>'
```

The import renews once before reporting success, so it doubles as the check.

## Data

In `WALLA_DATA_DIR`:

| File | Holds |
| :--- | :--- |
| `session.json` | The session |
| `last_run.json` | The last pass |

## CLI

```bash
wallapop-catalogue serve                   # the daily pass, the bot's commands, /healthz
wallapop-catalogue run --dry-run           # what a pass would reactivate
wallapop-catalogue session show | refresh  # check the session
```

## Configuration

| Variable | Default | |
| :--- | :--- | :--- |
| `WALLA_TELEGRAM_TOKEN` | – | @PlatanosServicesBot's token (empty = no commands) |
| `WALLA_TELEGRAM_CHAT` | – | Owner's chat, the only one answered |
| `WALLA_INTERVAL` | `24h` | Time between passes |
| `WALLA_RETRY_EVERY` | `15m` | Retry after a failed pass |
| `WALLA_MIN_PAUSE` / `MAX_PAUSE` | `20s` / `90s` | Pause between reactivations |
| `WALLA_MAX_PER_RUN` | `25` | Listings touched per pass |
| `WALLA_WARN_BEFORE` | `72h` | `/healthz` warns this long before the cookie expires |
| `WALLA_DATA_DIR` | `./data` | Data folder |
| `WALLA_PORT` | `8000` | `/healthz` port |
| `WALLA_PUSHGATEWAY` | – | Where metrics go |
| `WALLA_LOG_JSON` | – | `1` for JSON logs |

Endpoints can be overridden without a rebuild: `WALLA_BASE_URL`, `WALLA_WEB_URL`,
`WALLA_PATH_ITEMS`, `WALLA_PATH_REACTIVATE`, `WALLA_REACTIVATE_METHOD`, `WALLA_SIGN_SCHEME`,
`WALLA_DEVICE_ID`, `WALLA_APP_VERSION`.

## Health

`/healthz` always answers 200 while the process runs. Its `status` is `down` without a valid
session, `warn` when the cookie is about to expire or the last pass failed, `ok` otherwise.

Metrics go to the Pushgateway under the job `wallapop-catalogue`: `wallapop_last_run_status`,
`wallapop_last_run_timestamp`, `wallapop_expired_listings`, `wallapop_reactivated_listings` and
`wallapop_session_days_remaining`.
