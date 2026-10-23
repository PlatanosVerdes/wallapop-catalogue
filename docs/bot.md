# The services bot

## The services bot

@PlatanosServicesBot is shared by the small services on the Pi, so every command carries the
service's prefix. Only the owner's chat (`WALLA_TELEGRAM_CHAT`) is answered.

| Command | What it does |
| :--- | :--- |
| `/wallapop_estado` | Last pass, next one, and how long the session can still renew itself |
| `/wallapop_reactivar` | A pass right now. It answers at once and reports when it ends |

Telegram hands each update to one reader only. With `WALLA_TELEGRAM_UPDATES=push`
[switchboard](https://github.com/PlatanosVerdes/switchboard) is that reader: it posts this service's
updates to `POST /telegram/update` and reads its menu from `GET /telegram/commands`. The default,
`poll`, has this service read the bot itself, which is also the way back if switchboard fails. Any
service can still send to the bot.

Nothing is sent on its own: a dead or expiring session and a pass that stopped running are Grafana
alerts, built on the metrics below.
