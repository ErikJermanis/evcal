server := "erik@erikjermanis.me"
remote_dir := "/home/erik/sites/evcal"
# Must match PORT in the server's .env.
remote_port := "8081"
stamp := `date -u +%Y%m%dT%H%M%SZ`

default: start

# Generate Go code from all .templ files (templ is pinned in go.mod).
generate:
    go run -mod=mod github.com/a-h/templ/cmd/templ generate

# Start locally at http://localhost:8081 (or set PORT). Restart after edits.
start: generate
    go run .

alias dev := start

# Print a one-use pairing link for this machine's local server.
pair name:
    go run . pair {{ quote(name) }}

# List devices paired with the local database.
devices:
    go run . devices

# Sign a device out of the local database.
revoke id:
    go run . revoke {{ id }}

test: generate
    go test ./...

# Create a self-contained binary with the embedded public/ assets.
build: generate
    mkdir -p bin
    go build -o bin/evcal .

buildforlinux: generate
    mkdir -p bin
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/evcal-linux .

push:
    git add .
    git commit -m "push"
    git push

# Test, back up the live database, upload the Linux binary, restart, and check health.
deploy: test buildforlinux
    ssh {{ server }} 'cd {{ remote_dir }} && if [ -x ./evcal ]; then ./evcal backup backups/evcal-{{ stamp }}.sqlite && ls -1t backups/*.sqlite | tail -n +31 | xargs -r rm --; fi'
    scp bin/evcal-linux {{ server }}:{{ remote_dir }}/evcal-tmp
    ssh {{ server }} 'mv {{ remote_dir }}/evcal-tmp {{ remote_dir }}/evcal'
    ssh {{ server }} 'sudo /usr/bin/systemctl restart evcal.service'
    ssh {{ server }} 'curl -fsS --retry 10 --retry-connrefused --retry-delay 1 http://127.0.0.1:{{ remote_port }}/healthz' && echo " — deployed"

# Print a one-use pairing link for the production server.
prod-pair name:
    ssh {{ server }} {{ quote("cd " + remote_dir + " && ./evcal pair " + quote(name)) }}

prod-devices:
    ssh {{ server }} 'cd {{ remote_dir }} && ./evcal devices'

prod-revoke id:
    ssh {{ server }} 'cd {{ remote_dir }} && ./evcal revoke {{ id }}'

# Download a consistent copy of the production database into backups/.
prod-backup:
    mkdir -p backups
    ssh {{ server }} 'cd {{ remote_dir }} && ./evcal backup backups/evcal-{{ stamp }}.sqlite'
    scp {{ server }}:{{ remote_dir }}/backups/evcal-{{ stamp }}.sqlite backups/

prod-logs:
    ssh {{ server }} 'journalctl -u evcal.service -n 100 --no-pager'
