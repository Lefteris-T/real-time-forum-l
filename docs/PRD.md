# Real-Time Forum — Product Requirements

## Purpose

Build a new single-page forum from the existing Go forum baseline. The new
project adds JavaScript-driven navigation, online presence, and real-time
private messages while retaining compatible forum and account behavior.

The product requirements come from `docs/exercise.md`; passing behavior is
checked using `docs/audit.md`. The implementation sequence is in
`docs/tasks.md`.

## Users And Access

Users must register and log in before accessing forum features. Registration
collects:

- nickname
- age
- gender
- first name
- last name
- email
- password

Login accepts either nickname or email together with the password. Passwords
are stored as bcrypt hashes, not plaintext. A user can log out from any SPA
view. Sessions use opaque identifiers and retain their state on the server.

Guests can access the registration and login experience. They cannot use
authenticated forum actions or establish an authenticated WebSocket
connection.

## Forum Content

- authenticated users can create categorized posts
- users can browse posts in a feed
- opening a post loads its comments; comments are not required in the feed
- authenticated users can comment on posts
- posts and comments persist in SQLite

The existing forum may provide reusable schema or behavior. The new forum
uses a separate fresh database file (for example, `data/realtime-forum.db`),
leaving the old database file untouched. Initialize the new file by applying
the existing numbered migrations, then append new numbered migrations for the
new requirements. Do not import old users, posts, comments, or other records.
This avoids inventing values for newly required account fields and makes the
data reset explicit and reversible while retaining the existing migration
history as a schema starting point.

## Single-Page Frontend

The user-facing application has one HTML document. JavaScript handles changes
between registration, login, feed, post detail, and chat views without loading
another HTML page. HTML defines structure and CSS defines presentation. Use
plain JavaScript; frontend libraries and frameworks are not allowed.

The chat contact section stays visible while navigating forum views.

## Presence And Contact List

- show users as online or offline
- users can start private messages with users who are online
- order contacts with prior messages by most recent message activity, as in a
  conversation list
- order users with no prior messages alphabetically
- update presence and contact state without a page refresh
- account for multiple active browser connections for one user when
  determining online/offline status

The final placement of the current user's own account in the contact list is
an implementation detail to settle before contact-list implementation.

## Private Messages

- conversations and messages persist in SQLite
- opening a contact loads the existing conversation history
- initially load the latest ten messages
- when the user scrolls upward, load ten older messages at a time
- throttle or debounce scroll-triggered loading to avoid duplicate requests
- preserve the user's reading position when older messages are prepended
- display the sender's name and the date/time sent for each message
- deliver new messages and notifications over WebSockets without refreshing
  the page
- users who disconnect can retrieve messages from persistent history later

Only conversation participants may read or send within that conversation.
Messages are persisted before live delivery is attempted, so a disconnect
does not erase a sent message.

The client sends a message through an authenticated HTTP endpoint. The service
validates the sender, recipient, and conversation, persists the message, and
then publishes a live event through the WebSocket hub. The recipient's
JavaScript receives that event over its WebSocket connection. HTTP is the
command path; WebSockets carry live delivery and presence events.

Each conversation represents one unordered user pair. Store the IDs as
`user_low_id` and `user_high_id`, require `user_low_id < user_high_id`, and
enforce uniqueness on the pair. Keep nullable `last_message_at` on the
conversation and update it in the same transaction as each message so contact
ordering is straightforward. Index the pair, conversation activity, and
`(conversation_id, created_at, id)` for history queries. Message history uses
`(created_at, id)` as a deterministic chronological order and pagination
cursor. Online presence lives in WebSocket hub memory; multiple connections for
one account keep it online until its final connection closes.

## Backend And Allowed Packages

Go handles HTTP, data access, and WebSockets. SQLite stores account, forum,
conversation, and message data. The allowed dependencies are:

- Go standard library
- Gorilla WebSocket
- SQLite (`sqlite3`)
- bcrypt
- `gofrs/uuid` or `google/uuid`

Do not add a frontend framework or another dependency outside the exercise
allowlist.

## Security And Error Behavior

- validate all input on the server
- use parameterized SQL and database constraints
- authenticate HTTP actions and WebSocket upgrades
- validate WebSocket origin and conversation membership
- escape user-provided text when displaying it
- handle malformed requests, WebSocket failures, and database errors without
  exposing secrets or internal implementation details
- never log passwords, session cookies, or private message bodies

## Mandatory Acceptance

The mandatory project must pass every applicable check in `docs/audit.md`:

- a new user can register with all required fields
- an unregistered user cannot log in
- a registered user can log in using nickname or email and can log out from
  every view
- authenticated users can create posts and comments, and see persisted
  content
- online/offline contacts are visible and ordered as specified
- messages display the sender's name and sent date
- two users in separate browsers exchange a live message and notification
- conversations initially show the latest ten messages, then load ten more as
  the user scrolls upward without spamming requests
- the package allowlist is respected

## Bonus Scope

Do not begin these until mandatory acceptance passes:

- user profiles
- images in private messages
- measured performance improvements and additional code-quality work

Video and screen sharing are mentioned as possibilities in the exercise
introduction but are not in its objectives or audit checklist, so they are not
part of the required release.
