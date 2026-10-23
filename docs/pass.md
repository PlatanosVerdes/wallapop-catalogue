# The pass and the session

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
