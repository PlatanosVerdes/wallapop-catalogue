# Operations

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
