# Real-Time Forum Project Guide

## Project

This repository is being extended from an earlier Go forum into the new
single-page, real-time forum described in `docs/exercise.md`. The new project
uses Go and SQLite on the backend, JavaScript and WebSockets in the browser,
one HTML page, and CSS. Reuse old forum code only after checking that it fits
the new behavior and security requirements.

The active work order is in `docs/tasks.md`. Complete all mandatory phases
before starting the separate bonus section.

## Source Of Truth

When requirements or guidance disagree, use this order:

1. `docs/exercise.md` — project requirements and allowed packages
2. `docs/audit.md` — functional passing checks
3. `docs/PRD.md` — clarified product behavior and fixed decisions
4. `docs/tasks.md` — implementation order and learning goals
5. existing code and tests — reusable baseline, subject to the new exercise

Do not silently reinterpret an exercise requirement. If a product or
architecture decision is genuinely unclear, document the decision in the PRD
and task plan before implementing dependent behavior.

## Mandatory Scope

- registration with nickname, age, gender, first name, last name, email, and
  password
- login by nickname or email plus password, and logout from every view
- authenticated access to forum features
- categorized posts, a feed, and comments shown when a post is opened
- one HTML document, with view changes handled in JavaScript
- SQLite persistence
- online/offline user presence and an always-visible chat contact section
- contacts ordered by latest message; users with no messages ordered
  alphabetically
- private conversation history, initially loaded ten messages at a time, with
  older messages loaded on scroll using throttling or debouncing
- messages show sender name and sent date
- WebSockets for live private messages and notifications without page refresh
- allowed-package compliance and a reproducible manual audit walkthrough

Live video and screen sharing are not required by the exercise objectives.
Profiles and images in private messages are bonus work. Do not start bonus work
until every mandatory audit check passes.

## Technology Constraints

- Go standard library
- Gorilla WebSocket
- SQLite driver (`sqlite3`)
- bcrypt
- `gofrs/uuid` or `google/uuid`
- no frontend frameworks or libraries such as React, Angular, or Vue
- no additional package unless the exercise's allowlist is deliberately
  updated

Before adding a dependency, inspect `go.mod` and verify it against the
exercise. Keep browser code in plain JavaScript, with no framework dependency.

## Architecture

Keep responsibilities at their natural boundaries:

```text
HTML/CSS/JavaScript single page
-> HTTP endpoints and authenticated WebSocket endpoint
-> handlers and middleware
-> application services
-> repositories
-> SQLite
```

- schema migrations own durable structure, foreign keys, and indexes
- repositories own parameterized SQL and persistence queries
- services own validation, authentication, authorization, and business rules
- HTTP handlers own request/response translation
- the WebSocket hub owns live connection registration, routing, and shutdown
- JavaScript owns SPA navigation, rendering, browser events, and socket events
- HTML provides the one page's structure; CSS provides presentation

Do not put business rules in DOM handlers or database queries in WebSocket
connection code. Add abstractions only where they give a useful test boundary.

## Implementation And Learning Rules

- follow the dependency order in `docs/tasks.md`: user persistence, account
  flows, forum content, then conversations and messages
- keep commits small, coherent, and explainable; split a task further if it
  combines unrelated behavior
- explain each change in plain language: requirement, owning layer, behavior,
  and failure cases
- add focused tests with each backend behavior; use concrete manual checks for
  browser and two-client behavior that is difficult to automate
- preserve unrelated working-tree changes
- inspect and test reused old code against this exercise instead of assuming it
  already meets the new requirements
- do not claim a command or behavior in documentation until it has been
  verified

## Security And Data Rules

- hash passwords with bcrypt; never persist or log plaintext passwords
- use opaque server-generated session identifiers and keep session state
  server-side
- authenticate HTTP actions and WebSocket upgrades
- validate input on the server, even when JavaScript validates it too
- use parameterized SQL, foreign keys, and transactions where related writes
  must succeed together
- verify conversation membership before reading history or sending a message
- validate WebSocket origin and handle malformed frames and disconnects safely
- escape user content when rendering HTML; do not inject untrusted content as
  markup
- return safe client errors without exposing SQL, filesystem paths, secrets,
  or internal stack details
- keep secrets, local databases, uploads, logs, and generated artifacts out of
  Git

## Baseline Preservation

The earlier forum is a source of reusable behavior, not a reason to preserve
incompatible architecture. During Phase 0, identify what should carry forward.
Preserve relevant authentication, password hashing, session handling, SQL
safety, and forum data where compatible. Do not let old server-rendered
multi-page assumptions override the new one-page JavaScript requirement.

## Completion

The mandatory project is complete when all required checks in `docs/audit.md`
pass, including registration/login/logout, posts/comments, contact ordering,
message formatting, two-browser live delivery, and ten-at-a-time history
loading. Bonus items remain explicitly separate and must not be represented as
mandatory audit requirements.
