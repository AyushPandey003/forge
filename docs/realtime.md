# Forge realtime workflow updates

## Implemented baseline

Solution lists, boards, timelines and approval inboxes refresh automatically
while a Solutions or Solution board page is mounted. One authenticated SSE
connection per mounted page receives payload-free invalidations; normal HTTP
requests fetch fresh, authorized snapshots. Unsaved document editors are not
modified.

Endpoint: `GET /v1/forge/live`.

Migration 36 adds per-user durable revision rows. Database triggers advance
those rows in the same transaction as solution, membership, artifact, approval,
timeline and linked document title/current-version changes. Rollbacks do not
publish invalidations. Current solution creators/members are recipients; a
removed member gets one final invalidation to discard stale access, then no
future revisions from that solution. Invalidations contain no solution IDs,
titles, document contents or event counts.

Each stream checks its revision every two seconds. On initial connection and
reconnection the server always sends an invalidation with the current revision
as the SSE ID. The browser debounces query invalidation by 200ms. Therefore
missed changes are recovered by fetching a fresh snapshot, even with a stale
Last-Event-ID. This deliberately does not replay individual business events.
A row increment (rather than a global sequence) serializes concurrent changes
for each recipient and avoids skipping late commits.

## Connection lifecycle

- SSE headers disable caching/transformation and request proxy buffering off.
- Heartbeat every 15 seconds.
- Five-second write deadlines where the server supports them.
- Stream lifetime capped at one minute so reconnection re-runs authentication.
- Browser EventSource handles reconnect; unmount closes the connection.
- Poll/database failures end the stream; reconnect refetches a snapshot.
- Current HTTP read permissions remain authoritative, including after removal.

## Capacity and limits

This is a bounded, local-first baseline, not a benchmarked scale claim. One
revision query per connected tab every two seconds is about N/2 queries/sec.
Every affected member's row is updated on a workflow mutation, so hot solutions
can contend on those rows. No Redis is required; separate API instances observe
the same durable state. Multiple tabs still open separate streams.

Before large deployment:
- Load test idle streams, hot solutions, slow readers and reconnect storms.
- Replace per-stream polling with a per-process shared watcher and
  LISTEN/NOTIFY wakeups plus periodic durable reconciliation.
- Use HTTP/2, disable proxy buffering, configure idle timeouts and stream caps.
- Add event lag/active stream/reconnect/DB-query metrics.
- Consider Redis fan-out only if measured requirements justify it.

Out of scope: simultaneous text editing, durable Stitch execution/job progress,
push while the user is outside Solutions pages, workspace-wide notification
center, exactly-once business-event delivery, immediate closure of an existing
stream on session revocation (bounded by one-minute lifetime). Local password
reset/logout prevents authenticated reads immediately; existing SSE streams
carry only payload-free invalidations until reconnection.

## Tests

- Store integration: create, rollback, join, removal, future-change exclusion,
  outsider privacy.
- HTTP integration: auth required, initial snapshot invalidation, live change,
  disconnected change and reconnect with an invalid/high cursor.
- Existing Go suite and frontend tests remain applicable.
