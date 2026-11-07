# evcal — Događaji

Private list of networking events, filled by pasting JSON from the daily ChatGPT event search. Go, [templ](https://templ.guide/), plain CSS and SQLite ([modernc.org/sqlite](https://modernc.org/sqlite), pure Go). Requires Go and [just](https://just.systems/); templ is pinned in `go.mod`. The binary embeds all templates and assets.

| Command | Purpose |
| --- | --- |
| `just start` (or `just dev`) | Generate templates and serve `http://localhost:8081`. Override with `PORT=3000`; restart after edits. |
| `just pair "Erik - Mac"` | Print a one-use pairing link for the local server. |
| `just devices` / `just revoke <id>` | List or sign out devices paired with the local database. |
| `just test` | Generate templates and run Go tests. |
| `just build` | Build `bin/evcal`. |
| `just deploy` | Test, back up the live database, upload the Linux binary, restart `evcal.service`, and check `/healthz`. |
| `just prod-pair "Erik - iPhone"` | Print a one-use pairing link for production. |
| `just prod-devices` / `just prod-revoke <id>` | Manage production devices. |
| `just prod-backup` | Download a consistent copy of the production database into `backups/`. |
| `just prod-logs` | Show the service's last 100 log lines. |

## Local testing

```bash
just start
just pair "Erik - Mac"   # in a second terminal; open the printed link
```

The local database is `data/evcal.sqlite`; delete `data/` to start over.

## Event format

See [PROMPT.md](PROMPT.md) for the ChatGPT prompt and the JSON fields. On **+ Dodaj**, paste one event, several, or a whole ChatGPT answer; every event found is saved. An event with the same start date and the same title or link as a saved one updates it. **Uredi** opens the event's JSON for editing.

## Device pairing

There are no accounts. `evcal pair <name>` creates a link valid for 24 hours and usable once. Opening it shows a confirmation button (so chat-app link previews cannot use it up); confirming stores a random session token in an `HttpOnly`, `SameSite=Lax` cookie (also `Secure` when `PUBLIC_BASE_URL` is https). The cookie is renewed on every visit, so a device stays paired as long as it is used at least once a year. Only SHA-256 hashes of tokens are stored. Revoking a device takes effect on its next request.

If the app is added to an iPhone home screen and does not share Safari's cookies, copy the pairing link without opening it and paste it into the field on the "Ovaj uređaj nije uparen" page inside the app.

## Production

Layout on the server (`erik@erikjermanis.me`):

```
/home/erik/sites/evcal/
  evcal          binary (uploaded by just deploy)
  .env           settings
  data/          SQLite database (created on first start)
  backups/       database copies (made before each deploy; 30 newest kept)
```

The service and the `just prod-*` commands both run as `erik` from that directory, so they share the `.env` and database.

### One-time setup

1. Create the directory and settings. `PORT` must match `remote_port` in the justfile.

   ```bash
   ssh erik@erikjermanis.me
   mkdir -p /home/erik/sites/evcal
   cat > /home/erik/sites/evcal/.env <<'EOF'
   PORT=8081
   DATABASE_PATH=data/evcal.sqlite
   PUBLIC_BASE_URL=https://evcal.erikjermanis.me
   EOF
   ```

2. Create `/etc/systemd/system/evcal.service`:

   ```ini
   [Unit]
   Description=evcal
   After=network.target

   [Service]
   Type=simple
   User=erik
   WorkingDirectory=/home/erik/sites/evcal
   ExecStart=/home/erik/sites/evcal/evcal
   Restart=on-failure
   RestartSec=3

   [Install]
   WantedBy=multi-user.target
   ```

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable evcal.service
   ```

3. Proxy the domain to the app, then add HTTPS (for example with `sudo certbot --nginx -d evcal.erikjermanis.me`):

   ```nginx
   server {
       listen 80;
       server_name evcal.erikjermanis.me;

       location / {
           proxy_pass http://127.0.0.1:8081;
           proxy_set_header Host $host;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }
   ```

4. Allow the deploy restart without a password: run `sudo visudo -f /etc/sudoers.d/evcal-deploy` and add exactly:

   ```sudoers
   erik ALL=(root) NOPASSWD: /usr/bin/systemctl restart evcal.service
   ```

5. From your laptop: `just deploy`, then `just prod-pair "Erik - iPhone"` and open the link on the phone.

### Backups

`just deploy` runs `evcal backup` on the server before replacing the binary (migrations run when the new version starts). `evcal backup <file>` uses SQLite's `VACUUM INTO`, which is safe while the server runs. `just prod-backup` copies a fresh backup to your laptop.
