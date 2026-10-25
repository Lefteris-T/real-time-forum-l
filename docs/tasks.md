# Real-Time Forum — Learning And Implementation Plan

## Goal

Build the new single-page forum described by `docs/exercise.md` and make it
pass the manual checks in `docs/audit.md`. Reuse suitable ideas or code from the
existing forum, but design for the new requirements: a JavaScript frontend,
WebSocket events, online presence, and private conversations.

Work in small commits. Each commit should implement one behavior, include its
focused tests or a documented manual check, and be explainable in plain
language. Finish every mandatory phase before starting bonus work.

## Source Of Truth

For this project, use this order when documents or existing code disagree:

1. `docs/exercise.md` — project requirements and allowed packages
2. `docs/audit.md` — functional passing checks
3. this task plan — implementation order and learning notes
4. existing code — reusable baseline, not an automatic specification

`docs/AGENTS.md` and `docs/PRD.md` describe the new JavaScript and WebSocket
project. Keep them aligned with the exercise, audit, and this plan as decisions
are made.

## Mandatory Scope

- registration with nickname, age, gender, first name, last name, email, and
  password
- login by nickname or email plus password, and logout from every view
- authenticated access to the forum
- categorized posts, feed display, and comments shown when a post is opened
- a single HTML page whose view changes are handled by JavaScript
- online/offline users and private conversations, visible while using the
  forum
- contact ordering by most recent message, then alphabetically for users with
  no messages
- chat history loaded in pages of ten, with throttled or debounced scroll
  loading
- sender name and sent date shown for each message
- WebSockets for live message delivery and notifications
- SQLite persistence and only the packages allowed by the exercise
- preservation of relevant existing forum behavior where it fits the new
  requirements

Live video and screen sharing are not in the required objectives. User
profiles, images in private messages, and the general bonus checks are deferred
until the mandatory audit passes.

## Working Method For Every Commit

1. State the requirement in one sentence and explain it in plain language.
2. Identify which layer owns it: schema/repository, service, HTTP/WebSocket,
   JavaScript, HTML, or CSS.
3. Make one small change that can be reviewed on its own.
4. Add a focused automated test when practical; otherwise write a concrete
   manual check in the commit notes.
5. Run the focused check and the relevant regression checks.
6. Review the diff and explain the behavior and failure cases.
7. Commit with a message describing that single behavior.

Suggested commit messages below are examples, not a requirement to combine
adjacent tasks. Split a phase further if its change is too large to explain in
a few minutes.

---

# Part I — Mandatory Requirements

## Phase 0 — Reconcile The Project Documents And Inspect The Baseline

### Learn

- distinguish the old forum's completed behavior from the new exercise
- identify which code can be reused safely and which assumptions have changed
- inspect the allowed dependency list before adding packages

### Tasks

- review `docs/AGENTS.md` and `docs/PRD.md` against the exercise and audit;
  reconcile any remaining differences
- record the exercise's source-of-truth order and mandatory/bonus boundary
- inspect the existing Go modules, schema, auth, templates, and routes
- record a baseline of existing behavior and its current test status
- decide which old modules are reusable and list any that need replacement

### Acceptance

- project instructions consistently describe the SPA and WebSocket exercise
- no implementation behavior has been changed in this documentation phase
- dependency choices fit `docs/exercise.md`

### Suggested Commit

```text
docs: define real-time forum project scope
```

## Phase 1 — Define The New Architecture And Database Model

### Learn

- map a browser action through JavaScript, HTTP/WebSocket, service, and SQL
- model relationships and constraints before writing handlers
- keep persistence and business rules out of presentation code

### Tasks

- sketch the single-page frontend, Go HTTP server, WebSocket hub, services,
  repositories, and SQLite responsibilities
- identify entities for users, posts, categories, comments, conversations,
  messages, and presence/session state
- define how a conversation is uniquely identified for a pair of users
- define ordering data for contacts and stable pagination for messages
- identify which existing schema and migrations can safely be reused
- give the new forum a separate database file and create its schema from
  scratch; do not import or overwrite old forum records

### Acceptance

- relationships, uniqueness rules, timestamps, and indexes are documented
- every audit feature has a likely owning layer
- no third-party package outside the exercise allowlist is planned

### Suggested Commit

```text
docs: design real-time forum data model
```

## Phase 2 — Establish The Single-Page Shell And Static Assets

### Learn

- understand how one HTML document can show multiple views
- separate HTML structure, CSS presentation, and JavaScript behavior
- serve static assets safely from Go

### Tasks

- create the single HTML entry page
- add the stylesheet and a small JavaScript entry point without a framework
- add named view containers for registration, login, feed, post detail, and
  chat
- implement basic client-side view switching while keeping the URL/history
  behavior simple and understandable
- serve the page and static assets from the Go application

### Acceptance

- navigation between placeholder views does not load another HTML page
- no frontend framework or extra HTML page is introduced
- static assets load from the application

### Suggested Commit

```text
feat: add single-page forum shell
```

## Phase 3 — Add User Persistence In Small Steps

### Learn

- write SQLite migrations with foreign keys and constraints
- use parameterized SQL and transactions
- distinguish repository work from service rules

### Commit 3.1 — Add The User Schema

- add a migration for nickname, age, gender, first name, last name, email, and
  password hash
- add required-field and unique nickname/email constraints
- apply the schema to the new forum database; do not backfill or import old
  account records

Acceptance: the new forum starts with a fresh schema in its own database file,
the old database remains untouched, and duplicate nickname/email values are
rejected.

```text
feat: add required account fields to schema
```

### Commit 3.2 — Add User Repository Operations

- implement create-user and lookup-by-nickname-or-email operations
- keep SQL parameterized and return predictable not-found/duplicate errors
- test persistence and uniqueness behavior

Acceptance: repository tests prove user data round-trips and duplicate
identifiers cannot be created.

```text
feat: add account repository operations
```

## Phase 4 — Registration, Login, And Logout In Small Steps

### Learn

- validate user input on the server even when the browser also validates it
- store password hashes, never plaintext passwords
- understand cookies and server-side sessions
- avoid account-existence leaks in login errors

### Commit 4.1 — Define Account Input Validation

- define server-side validation for all required fields
- choose sensible age bounds and normalize nickname/email consistently
- return field-specific validation errors without touching persistence

Acceptance: focused tests cover missing, malformed, and boundary values.

```text
feat: validate account registration input
```

### Commit 4.2 — Hash Passwords At The Service Boundary

- retain bcrypt for password creation and verification
- add focused tests for successful comparison and wrong-password rejection
- make sure repository APIs accept only the password hash, not plaintext

Acceptance: tests show stored credentials are bcrypt hashes and plaintext is
not included in errors or logs.

```text
feat: hash account passwords with bcrypt
```

### Commit 4.3 — Register Accounts

- connect validated account input, password hashing, and user persistence
- map duplicate nickname/email errors to safe field feedback
- keep registration independent from login/session creation at first

Acceptance: registration persists all required fields and rejects duplicate
identifiers without creating a partial account.

```text
feat: register forum accounts
```

### Commit 4.4 — Authenticate By Nickname Or Email

- look up the account using either identifier
- compare the password with bcrypt
- return one generic invalid-credentials result for unknown account and wrong
  password

Acceptance: audit login works for both identifier types and fails for an
unregistered user.

```text
feat: log in with nickname or email
```

### Commit 4.5 — Establish And Validate Sessions

- retain or adapt opaque server-side sessions and secure cookie attributes
- reject missing, malformed, and expired sessions
- test session creation, lookup, expiry, and replacement behavior

Acceptance: the cookie contains an opaque identifier while user/session state
remains server-side.

```text
feat: establish server-side login sessions
```

### Commit 4.6 — Add Logout And Route Protection

- invalidate the session on logout
- protect authenticated HTTP actions and future WebSocket upgrades
- allow only registration/login assets and endpoints to guests

Acceptance: unauthenticated users cannot perform forum actions and logging out
invalidates access.

```text
feat: protect forum routes and support logout
```

### Commit 4.7 — Connect The SPA Registration Form

- build the registration form with all required fields
- submit it to the Go endpoint and display server validation results

```text
feat: connect SPA registration form
```

### Commit 4.8 — Connect The SPA Login And Logout Controls

- build login with a single nickname-or-email field and password
- show the authenticated view after success and login view after logout
- make the logout control available from each SPA view

Acceptance: the audit registration, login, and logout checks pass through the
browser interface.

```text
feat: connect SPA login and logout
```

## Phase 5 — Categories, Posts, Feed, And Comments

### Learn

- connect a server-rendered data model to a client-rendered view
- distinguish a list/feed query from a detail query
- validate ownership/authentication at service boundaries

### Commit 5.1 — Add Category And Post Schema

- add or adapt category and post tables with foreign keys
- add indexes for category lookup and newest-first feed ordering
- Reuse suitable repository and service code where it fits; store new forum data in the separate database.

Acceptance: posts reference valid authors and categories, with migration tests
for those constraints.

```text
feat: add category and post schema
```

### Commit 5.2 — Add Category And Post Repository Operations

- add category listing and post create/feed queries
- keep SQL parameterized and define stable newest-first ordering
- test persistence, category association, and feed ordering

```text
feat: add category and post repositories
```

### Commit 5.3 — Add Comment Schema And Repository

- add comments with post and author foreign keys
- add a stable chronological ordering key/index
- add create and list-by-post queries
- test invalid references and comment ordering

```text
feat: add comment persistence
```

### Commit 5.4 — Create Categorized Posts

- implement category listing and categorized post creation
- validate authenticated author and selected category

```text
feat: add categorized post creation
```

### Commit 5.5 — Show The Feed

- implement the feed query and render posts in the JavaScript view

```text
feat: show posts in the SPA feed
```

### Commit 5.6 — Load Post Details And Comments On Demand

- load post details on click
- request and display comments only after a post is opened

```text
feat: load post details and comments on demand
```

### Commit 5.7 — Create Comments

- create comments for authenticated users
- show safe errors and empty states without exposing internal failures

Acceptance: audit post creation, feed visibility, commenting, and post detail
checks pass; posts and comments remain available after restart.

```text
feat: add comments to posts
```

### Acceptance

- audit post creation, feed visibility, commenting, and post detail checks pass
- posts and comments remain available after restart
- comment content is not requested until a post is opened

## Phase 6 — WebSocket Connection And Connection Lifecycle

### Learn

- understand the HTTP upgrade handshake
- understand goroutines, channels, and safe concurrent writes
- distinguish a connected socket from an authenticated user

### Tasks

- add an authenticated WebSocket endpoint using the allowed Gorilla package
- validate the origin and associate the connection with the logged-in user
- design a hub/client lifecycle for registration, disconnect, and shutdown
- ensure one failed or disconnected client does not block other users
- add a small message envelope/type scheme for future presence and chat events
- handle malformed frames and unexpected disconnects safely

### Acceptance

- only authenticated users can connect
- multiple clients can connect and disconnect cleanly
- concurrent writes are serialized safely per connection
- WebSocket lifecycle behavior has focused tests

### Suggested Commits

```text
feat: add authenticated websocket endpoint
feat: manage websocket client lifecycle
```

## Phase 7 — Online/Offline Presence And Contact Ordering

### Learn

- distinguish process-local live presence from durable user/message data
- model multiple browser connections for the same user
- compute contact ordering from conversation activity

### Commit 7.1 — Add Conversation And Message Schema

- represent a private conversation between a unique pair of users
- add messages with sender, conversation, and timestamp fields
- add indexes for latest conversation activity and older-message pagination
- decide a stable cursor/order for messages with equal timestamps

Acceptance: constraints prevent duplicate pair conversations and message
ordering is deterministic.

```text
feat: add private conversation schema
```

### Commit 7.2 — Add Conversation And Message Repository Operations

- create or find a conversation for a user pair
- query latest conversation activity for contact ordering
- query latest ten messages and older messages using the chosen cursor
- test participant constraints, persistence, and deterministic ordering

```text
feat: add private message repository
```

### Commit 7.3 — Track Online And Offline State

- track active connections safely and define when a user becomes online/offline
- support multiple connections for one account without marking them offline
  until the last connection closes
- send presence changes over WebSockets and update the JavaScript view

```text
feat: track online forum users
```

### Commit 7.4 — Show And Order Chat Contacts

- expose the online/offline contact list in the always-visible chat section
- sort users with conversations by their latest message time
- sort users with no messages alphabetically
- document whether the current user's own account appears in the list

### Acceptance

- presence changes without a page refresh
- contact list remains visible across forum views
- ordering matches the audit for message and no-message users

```text
feat: show ordered chat contacts
```

## Phase 8 — Private Conversation History And Ten-Message Pagination

### Learn

- use stable cursor/keyset pagination rather than offset assumptions where
  practical
- preserve scroll position when prepending older messages
- throttle or debounce scroll-triggered requests

### Tasks

- open a conversation when a contact is selected
- load the latest ten messages first, in chronological display order
- fetch ten older messages on upward scroll, with a stable cursor
- prevent duplicate/in-flight fetches and stop when history is exhausted
- throttle or debounce the scroll handler
- preserve the reader's position when older messages are inserted
- show a loading state and recover from failed history requests

### Acceptance

- conversations with over ten messages initially show only the latest ten
- scrolling up loads ten at a time without repeated duplicate requests
- conversations over twenty messages can be read back to their beginning

### Suggested Commits

```text
feat: load latest private messages
feat: paginate older chat history
fix: throttle chat history scroll loading
```

## Phase 9 — Persist And Deliver Private Messages In Real Time

### Learn

- persist a message before notifying recipients
- use channels to route events without coupling WebSocket code to SQL
- distinguish delivery notifications from message history

### Tasks

- allow an authenticated user to message another online user
- validate recipient, message content, and conversation membership
- persist each message with sender, recipient/conversation, and timestamp
- broadcast a new-message event to the recipient's active connections
- update the sender's view and contact ordering without a refresh
- send a generic notification when a message arrives in a closed conversation
- render sender name and date for both historical and live messages
- define behavior for a recipient who disconnects during send; persisted
  history must be available on the next open

### Acceptance

- two browsers logged in as different users exchange messages live
- recipient sees a notification without refreshing
- message shows sender name and date
- persisted messages load correctly after reconnect/restart

### Suggested Commits

```text
feat: persist private chat messages
feat: deliver chat messages over websockets
feat: notify users of incoming messages
```

## Phase 10 — Integrate, Handle Failures, And Complete Mandatory Audit

### Learn

- follow a request from browser to database and back
- debug network and WebSocket behavior using browser tools
- distinguish automated regression coverage from the two-browser audit

### Tasks

- check allowed dependencies and remove accidental disallowed packages
- review authorization on every post, comment, message, and WebSocket action
- review HTML escaping, SQL parameterization, and safe error responses
- ensure logout closes or invalidates the user's WebSocket connection
- verify behavior for reconnect, duplicate events, send failures, and empty
  history
- complete the full checklist in `docs/audit.md` manually
- document local run instructions and the two-browser real-time walkthrough
- fix issues found by the audit before starting bonus work

### Definition Of Mandatory Done

- every required exercise behavior has an implementation and a passing audit
  check
- registration, login, logout, posts, comments, presence, private history, and
  live messaging all work
- history loads ten messages at a time using throttled/debounced scrolling
- all packages fit the allowlist
- existing baseline behavior selected for reuse still passes its regression
  checks
- the project can be explained commit by commit

### Suggested Commit

```text
docs: add real-time forum audit walkthrough
```

---

# Part II — Bonus Work

Start this part only after the mandatory definition of done is met. Pick bonus
items individually and keep them out of the mandatory acceptance claim.

## Bonus A — User Profiles

### Learn

- separate public profile fields from private account data
- authorize profile edits to the account owner

### Tasks And Acceptance

- define which profile fields are public
- add profile viewing and owner-only editing
- ensure passwords, email if private, session data, and other secrets are not
  exposed
- test profile access and editing permissions

### Suggested Commit

```text
feat: add user profiles
```

## Bonus B — Images In Private Messages

### Learn

- validate uploaded file content and size rather than trusting its filename
- store uploads safely and serve them with correct content types

### Tasks And Acceptance

- define allowed image formats and size limits
- validate and store images safely
- associate an image with a private message without breaking text messages
- display images only to conversation participants
- test invalid files, size limits, authorization, and persistence

### Suggested Commit

```text
feat: support images in private messages
```

## Bonus C — Performance And Code Quality

### Learn

- measure before optimizing
- use concurrency to solve actual waiting or throughput problems
- keep goroutine and channel lifetimes bounded

### Tasks And Acceptance

- profile database and browser request patterns
- remove demonstrably unnecessary requests and queries
- review indexes and query plans for feed, contacts, and history
- use goroutines/channels or JavaScript Promises where they make work clearer
  or more responsive
- run race checks and inspect goroutine shutdown behavior
- record a before/after result for any performance optimization

### Suggested Commit

```text
perf: reduce measured forum request overhead
```

## Bonus D — General Quality Review

Review usability, accessibility, error recovery, maintainability, and the
overall two-browser experience. Make small fixes with focused evidence rather
than adding features without a clear user or audit benefit.

### Suggested Commit

```text
refactor: improve forum usability and maintainability
```
