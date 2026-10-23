<p align="center">
  <img src="assets/icon.jpg" width="96" alt="">
</p>

<h1 align="center">wallapop-catalogue</h1>

<p align="center">
  <a href="https://github.com/PlatanosVerdes/wallapop-catalogue/tags"><img src="https://img.shields.io/github/v/tag/PlatanosVerdes/wallapop-catalogue?label=release&color=2f7d3a" alt="Latest release"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/PlatanosVerdes/wallapop-catalogue" alt="Go version">
</p>

Reactivates my expired Wallapop listings once a day, and answers for it in
[@PlatanosServicesBot](https://t.me/PlatanosServicesBot). It reads the catalogue, reactivates each
expired listing with a random pause between them, and reports the last pass on request.

Runs on the Raspberry from [rpi-services](https://github.com/PlatanosVerdes/rpi-services). Go, no
dependencies, CalVer tag on every push to main. The searches bot for friends is a separate service,
[wallabot](https://github.com/PlatanosVerdes/wallabot).

## Running it

```bash
export WALLA_TELEGRAM_TOKEN=<bot token>
export WALLA_TELEGRAM_CHAT=<owner chat id>
wallapop-catalogue session import --cookie '<value>'   # the browser session, see docs/pass.md
wallapop-catalogue serve                               # the daily pass, the bot's commands, /healthz
```

In production it is the image `ghcr.io/platanosverdes/wallapop-catalogue`; the compose entry is in
[deploy/docker.md](deploy/docker.md). Commands: `/wallapop_estado` and `/wallapop_reactivar`.

| Variable | Default | |
| :--- | :--- | :--- |
| `WALLA_TELEGRAM_TOKEN` | – | @PlatanosServicesBot's token (empty = no commands) |
| `WALLA_TELEGRAM_CHAT` | – | Owner's chat, the only one answered |
| `WALLA_INTERVAL` | `24h` | Time between passes |
| `WALLA_DATA_DIR` | `./data` | Data folder |

## Docs

| | |
| :--- | :--- |
| [The services bot](docs/bot.md) | The shared Telegram bot and its commands |
| [The pass and the session](docs/pass.md) | What a pass calls, and how to import the browser session |
| [Operations](docs/operations.md) | Data files, the CLI, every setting, health and metrics |
| [Deploy](deploy/docker.md) | The compose entry used on the Pi |
