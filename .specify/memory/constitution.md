<!--
Sync Impact Report
==================
Version change: TEMPLATE → 1.0.0 (initial ratification)
Modified principles: All five placeholders replaced with concrete principles.
Added sections:
  - Core Principles (5 principles)
  - Technology & Architecture Constraints
  - Development Workflow & Quality Gates
  - Governance
Removed sections: None (template placeholders only).
Templates requiring updates:
  - ✅ .specify/templates/plan-template.md — Constitution Check section will be
    populated by /speckit-plan at feature time; principle gates referenced below.
  - ✅ .specify/templates/spec-template.md — No changes required; spec remains
    technology-agnostic per principle separation.
  - ✅ .specify/templates/tasks-template.md — Foundational phase already covers
    schema, error handling, logging; principle-driven task types align.
  - ⚠ docs/quickstart.md — Not present yet; create when feature plans are first
    generated.
  - ✅ CLAUDE.md — Points to current plan; no constitution-specific edits needed.
Follow-up TODOs: None.
-->

# Beeyond Flash Sale Constitution

## Core Principles

### I. Consistency Over Availability (NON-NEGOTIABLE)

The system MUST prioritize strong consistency over availability in every code
path that touches inventory state. When the database is unreachable, a replica
is lagging, or a lock cannot be acquired within the configured timeout, the
service MUST reject the request with a deterministic error rather than serve
stale data, retry blindly, or fall back to optimistic guesses. Over-selling is
the defining failure mode of a flash sale system; any design that trades
correctness for throughput is rejected.

**Rationale**: A flash sale's value depends on the promise that "sold out" means
sold out. One oversell event is more costly — in refunds, support load, and
brand trust — than thousands of denied requests during peak load.

### II. Pessimistic Locking for Inventory Mutations (NON-NEGOTIABLE)

Every decrement of available inventory MUST occur inside a PostgreSQL
transaction that holds a row-level lock (`SELECT ... FOR UPDATE`) on the
relevant inventory row before issuing the update. Optimistic concurrency
control (version columns, CAS loops, `ON CONFLICT DO UPDATE` against stock
counts) is forbidden for inventory writes. Lock acquisition MUST have an
explicit timeout; expired waits MUST surface as a typed error, never a silent
retry.

**Rationale**: Under heavy contention, optimistic strategies either retry-storm
the database or admit subtle race windows. Pessimistic row locks serialize
contending writers deterministically and make the correctness argument
auditable from the SQL alone.

### III. Normalized Relational Schema

The database schema MUST be in third normal form (3NF) or stricter. Every
foreign key MUST be declared with `REFERENCES` and an appropriate `ON DELETE`
clause; every invariant expressible as a `CHECK`, `UNIQUE`, or `NOT NULL`
constraint MUST be declared at the schema level rather than enforced solely in
application code. Denormalization (cached counts, materialized totals,
duplicated columns) is forbidden in the initial implementation; if profiling
later demonstrates a bottleneck, the denormalization MUST be justified in the
plan's Complexity Tracking table and gated behind a triggered or transactional
mechanism that preserves the consistency guarantees of Principles I and II.

**Rationale**: Normalized schemas make invariants explicit and let PostgreSQL
defend correctness even when application code has bugs. For a flash-sale
workload measured in seconds, the cost of an extra join is negligible compared
to the cost of debugging a derived counter that drifted.

### IV. Strict Stack Discipline

The technology stack is fixed for the lifetime of the project:

- **Backend**: Go. Permitted HTTP routers are the standard library
  `net/http`, `github.com/go-chi/chi`, or `github.com/gin-gonic/gin` — no
  other web framework. Permitted database driver is `github.com/jackc/pgx`
  (or the `database/sql` adapter built on it). No ORM.
- **Database**: PostgreSQL, run locally via the project's
  `docker-compose.yml`. No alternative databases, no in-memory substitutes
  for inventory state.
- **Frontend**: React with TypeScript, built with Vite. No alternative
  framework, build tool, or JavaScript-only code in the frontend.

Introducing any other language, framework, datastore, or router requires a
constitution amendment (see Governance).

**Rationale**: Stack discipline keeps the codebase small enough that one
engineer can audit the entire consistency story end-to-end. Every additional
runtime is another place a race can hide.

### V. Concurrency Correctness Is Tested, Not Assumed (NON-NEGOTIABLE)

Every feature that mutates inventory MUST ship with an integration test that
launches concurrent clients against a real PostgreSQL instance (the same
Docker Compose service used for development) and asserts that the final stock
count matches the expected value and that no client received a confirmed
purchase beyond available stock. Mocks, fakes, or single-threaded tests do
NOT satisfy this requirement. Pull requests that change locking, transaction
boundaries, or SQL touching inventory MUST re-run these tests in CI and
MUST NOT merge on a green-only-because-skipped result.

**Rationale**: The system's central claim is "no oversells under concurrency."
That claim is only credible if it is exercised by tests that actually create
concurrency. Unit tests against mocks cannot falsify a locking bug.

## Technology & Architecture Constraints

- **Local development environment**: `docker compose up` MUST be sufficient to
  bring up PostgreSQL and run the backend and frontend against it. No
  developer setup may depend on a cloud database or shared environment.
- **Migrations**: Schema changes MUST be applied via versioned, append-only
  migration files checked into the repository. Hand-edited schemas are
  forbidden.
- **Transactions at the boundary**: HTTP handlers that mutate inventory MUST
  open a transaction at the handler entry, perform all reads/writes inside
  it, and commit/rollback before responding. Business logic functions MUST
  accept a transaction handle rather than open their own.
- **Errors are typed**: Lock-timeout, sold-out, and validation errors MUST be
  distinguishable in the API response (status code + machine-readable error
  code), so the frontend can render correct messaging without parsing
  English text.
- **Frontend never holds authoritative state**: The React client MUST treat
  the backend as the single source of truth for stock; client-side counters,
  optimistic UI updates of inventory, and local-storage caches of stock are
  forbidden.

## Development Workflow & Quality Gates

- **Plan-before-code**: Every feature begins with a `/speckit-plan` artifact
  that includes a Constitution Check confirming the five principles above
  are upheld. Violations MUST be enumerated in Complexity Tracking with a
  justification, or the plan MUST be revised before implementation.
- **Code review**: Pull requests MUST be reviewed against this constitution.
  Reviewers MUST explicitly confirm Principles I, II, and V for any PR
  touching inventory paths.
- **CI gates**: Backend builds, `go vet`, `go test ./... -race`, frontend
  type-checking (`tsc --noEmit`), and the concurrency integration suite
  MUST pass before merge.
- **Migrations forward-only**: Migrations MUST be tested by applying them to
  a freshly initialized database in CI. Down-migrations are not required.

## Governance

- This constitution supersedes all other practices, READMEs, and informal
  conventions in the repository. Where a doc and the constitution conflict,
  the constitution wins until the conflicting doc is updated.
- Amendments require: (a) a PR that edits this file, (b) a corresponding
  version bump per the policy below, and (c) a Sync Impact Report at the top
  of this file enumerating downstream template/doc changes.
- **Versioning policy**: Semantic versioning applied to governance.
  - **MAJOR**: Removing or fundamentally redefining a principle (e.g.,
    relaxing Principle I or II).
  - **MINOR**: Adding a new principle or section, or materially expanding
    an existing one.
  - **PATCH**: Clarifications, wording, typo fixes, non-semantic edits.
- **Compliance review**: At the start of each feature plan, the author MUST
  re-read this constitution and record the Constitution Check result in the
  plan. Drift discovered during review MUST be filed as a follow-up task.
- **Runtime guidance**: Day-to-day technical context (stack versions, shell
  commands, project layout) lives in the current feature plan, per
  `CLAUDE.md`. This constitution stays principle-level.

**Version**: 1.0.0 | **Ratified**: 2026-05-22 | **Last Amended**: 2026-05-22
