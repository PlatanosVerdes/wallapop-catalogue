# Deploy

Deployed by [rpi-services](https://github.com/PlatanosVerdes/rpi-services) from the image on GHCR,
pinned to the CalVer tag that `release.yml` creates on every push to main.

Entry for its `docker-compose.yml`:

```yaml
  # --- Wallapop catalogue: reactivate my expired listings daily ---
  wallapop-catalogue:
    image: ghcr.io/platanosverdes/wallapop-catalogue:${WALLAPOP_CATALOGUE_VERSION}
    container_name: wallapop-catalogue
    labels:
      prometheus.probe: "http://wallapop-catalogue:8000/healthz"
    restart: unless-stopped
    profiles: [wallapop, all]
    mem_limit: 64m
    memswap_limit: 64m
    dns:
      - 8.8.8.8
      - 1.1.1.1
    networks:
      - media-network
    environment:
      TZ: ${TZ:-Europe/Madrid}
      WALLA_PUSHGATEWAY: http://pushgateway:9091
      # Shared with other services; this one is its only reader of updates.
      WALLA_TELEGRAM_TOKEN: ${TELEGRAM_BOT_SERVICES}
      WALLA_TELEGRAM_CHAT: ${WALLA_TELEGRAM_CHAT}
    volumes:
      - ${APP_CONFIG_PATH}/wallapop-catalogue:/data
```

The session is the only manual step, and only when the cookie finally expires:

```bash
docker exec wallapop-catalogue wallapop-catalogue session import --cookie '<the __Secure-next-auth.session-token value>'
docker exec wallapop-catalogue wallapop-catalogue session show
```
