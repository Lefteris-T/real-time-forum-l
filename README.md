# Real-Time Forum

A single-page forum built with Go, SQLite, plain JavaScript, and WebSockets.
It supports registration and login, categorized posts and comments, live user
presence, private conversations, persisted message history, unread counts, and
live message delivery.

## Requirements

- Go 1.25 or newer
- A C compiler for the SQLite driver
- OpenSSL for the local HTTPS certificate
- Docker Compose only if you want to use the container workflow

The Go dependencies are limited to the exercise allowlist: the standard
library, SQLite driver, bcrypt, Google UUID, and Gorilla WebSocket. Browser code
uses plain JavaScript without a framework.

## Run Locally

Generate a localhost certificate. The script refuses to overwrite an existing
certificate or private key:

```sh
./scripts/generate-cert.sh
```

Copy the example environment file if you do not already have a local `.env`:

```sh
cp .env.example .env
```

Load the values and start the server from the repository root:

```sh
set -a
. ./.env
set +a
go run ./cmd/forum
```

Open <https://localhost:8443>. The local certificate is self-signed, so your
browser will show a warning unless you trust that certificate for local
development. The app creates its SQLite database at
`data/realtime-forum.db` and applies pending migrations on startup. Keep this
new database separate from any older forum database.

For a temporary plain-HTTP run, set these values instead of the HTTPS values:

```sh
FORUM_ADDRESS=localhost:8080 \
FORUM_DATABASE_PATH=data/realtime-forum.db \
FORUM_HTTPS_ENABLED=false \
FORUM_SECURE_COOKIE=false \
go run ./cmd/forum
```

Then open <http://localhost:8080>. Use HTTPS for the browser security and
WebSocket audit.

Stop the local server with Ctrl+C. Run automated Go checks with:

```sh
go test ./...
```

## Two-Browser Live Messaging Walkthrough

Use two separate browser profiles or one regular and one private window. Create
and log in to a different account in each browser; the app allows one active
session per account, so do not use the same account in both.

1. Confirm that each browser shows the other account as online in Contacts.
2. In the first browser, select the second user, enter a message, and send it.
3. Confirm it appears in the sender's chat and in the recipient's browser
   without refreshing. The recipient sees it in the open conversation, or sees
   a generic notice and unread count in Contacts if the conversation is closed.
4. Open the conversation in the recipient browser and confirm the unread count
   clears. Verify the message shows its sender and timestamp.
5. To check offline delivery, log out of the recipient, send another message,
   and log the recipient back in. Contacts should show the unread notice and
   count; opening the conversation should show the persisted message and clear
   the unread state.
6. To check history pagination, exchange at least 21 messages, reopen the
   conversation, and scroll upward. The latest ten load first; older messages
   load in pages of ten while the reading position stays in place.
7. Log out from the feed and chat views. The logged-out browser should return
   to the login screen and its WebSocket connection should close.

The manual acceptance checklist is in [`docs/audit.md`](docs/audit.md). Its
two-browser checks complement the automated tests; they do not run as part of
`go test ./...`.

## Docker Compose

Generate the certificate and create `.env` as described above. Set
`FORUM_CERT_GID` in `.env` to the output of `id -g`, then start the container:

```sh
docker compose up --build
```

The application is available at <https://localhost:8443>. SQLite data is kept
in the `forum-data` named volume. Stop the container without deleting that
data with:

```sh
docker compose down
```

## Project Notes

- Product and architecture decisions: [`docs/PRD.md`](docs/PRD.md)
- Implementation phases: [`docs/tasks.md`](docs/tasks.md)
- Manual acceptance checklist: [`docs/audit.md`](docs/audit.md)
- Exercise requirements: [`docs/exercise.md`](docs/exercise.md)
