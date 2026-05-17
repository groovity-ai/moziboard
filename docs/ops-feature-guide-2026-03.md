# MoziBoard Ops Feature Guide — March 2026

Last updated: 2026-03-11

This document explains the MoziBoard **Ops** feature in detail: what problem it solves, what data it shows, what actions are available, how the maintenance flow works, and how to use it safely during debugging and operations.

---

## 1. What Ops Is

MoziBoard Ops is the lightweight operational control surface for the agent/runtime layer.

It exists to answer questions like:
- apakah agent masih benar-benar online?
- kenapa event ke runtime gagal?
- connector mana yang sehat / disabled / error?
- apakah ada retry backlog?
- apakah task, agent run, dan agent state mulai drift?
- apakah sistem perlu dibersihkan / direconcile sekarang?

In short:
- **Boards / Tasks / Docs** = product work surface
- **Ops** = runtime health, delivery debugging, connector control, and maintenance surface

Ops is especially useful when MoziBoard is acting as a **mission control layer** for connected agents and runtimes.

---

## 2. Why Ops Matters

As soon as agent workflows become asynchronous, several classes of issues can appear:

### Common operational problems
- agent heartbeat stops, but the UI still looks alive
- event delivery fails and stays pending/failed
- runtime crashes mid-task
- task status says `done`, but run still says `in_progress`
- connector is technically registered, but is no longer usable
- retries pile up and nobody notices until work silently stalls

The Ops feature exists to make these issues:
1. **visible**
2. **actionable**
3. **recoverable**

---

## 3. Current Ops Scope

As of this version, Ops includes:

### Backend capabilities
- `GET /api/ops/summary`
- `GET /api/ops/agent-events`
- `POST /api/ops/agent-events/:id/retry`
- `POST /api/ops/agent-events/:id/requeue`
- `GET /api/ops/connectors`
- `POST /api/ops/connectors/:id/enable`
- `POST /api/ops/connectors/:id/disable`
- `GET /api/ops/audit-logs`
- `POST /api/ops/maintenance/run`

### UI capabilities
- board ops page: `/board/[id]/ops`
- ops summary panel
- event delivery table
- connector table
- event detail drawer
- connector detail drawer
- manual **Run maintenance** button

### Automation capabilities
- scheduled maintenance sweep via `OPS_MAINTENANCE_EVERY`
- current deployed default: `10m`

---

## 4. Conceptual Model

Ops is built around 5 operational entities:

### A. Agents
Represent the runtime worker identity.

Examples of things tracked:
- online/offline state
- heartbeat freshness
- current activity
- health note / issue hint

### B. Agent Runs
Represent active or historical execution runs tied to task work.

Examples:
- running
- blocked
- review
- failed
- stale / hung execution

### C. Agent Events
Represent outbound delivery work items that need to be sent to a runtime/connector.

Examples:
- task assigned
- task updated
- review request
- internal runtime push

These are the main things you inspect when delivery is misbehaving.

### D. Connectors
Represent how MoziBoard reaches a runtime or remote agent.

Examples:
- internal transport
- webhook transport
- shared-secret or bearer-token-backed connector
- connected / disabled connector state

### E. Audit Logs
Represent structured trace of important operational mutations.

Examples:
- retry requested
- requeue requested
- connector enabled/disabled
- maintenance sweep ran

---

## 5. Ops UI Overview

Current UI lives at:

```text
/board/[id]/ops
```

Even though the page is board-scoped in route structure, the current summary panel is still **global runtime-oriented**. Event list can already be filtered by `board_id`, while the summary endpoint is currently global.

### Main sections on the page
1. **Ops Summary**
2. **Quick Summary Cards**
3. **Event Delivery table**
4. **Connectors table**
5. **Detail drawers**
6. **Run maintenance action**

---

## 6. Ops Summary Panel

The summary panel is intended as a fast “what is happening right now?” snapshot.

Endpoint used:

```http
GET /api/ops/summary
```

### Returned groups
- `agents`
- `runs`
- `events`
- `connectors`
- `audit_logs`

### A. Agent summary
Fields:
- `total`
- `online`
- `offline`
- `stale`
- `blocked`
- `with_issues`

What they mean:
- **total**: total agent records known to the system
- **online**: agents currently marked online
- **offline**: agents currently marked offline
- **stale**: agents whose heartbeat is older than the staleness threshold and are still not cleanly offline
- **blocked**: agents marked blocked
- **with_issues**: agents with a non-empty health note or issue hint

### B. Run summary
Fields:
- `total`
- `active`
- `stale`
- `failed`
- `review`
- `blocked`

What they mean:
- **active**: runs still considered in-flight
- **stale**: runs that have exceeded the stale timeout window
- **failed**: runs that ended in failure
- **review**: runs currently waiting for review
- **blocked**: runs blocked by some issue

### C. Event summary
Fields:
- `pending`
- `processing`
- `failed`
- `dead`
- `sent`
- `routed`
- `ignored`
- `retry_due`

What they mean:
- **pending**: waiting to be claimed for delivery
- **processing**: currently being worked on by delivery worker
- **failed**: delivery failed but may still retry
- **dead**: exhausted retry policy / dead-letter-ish state
- **sent**: successfully sent to destination
- **routed**: handed off to pull/internal runtime flow
- **ignored**: explicitly ignored / non-actionable route outcome
- **retry_due**: events eligible to retry now

This is one of the most important signals for production debugging.

### D. Connector summary
Fields:
- `total`
- `connected`
- `disabled`
- `webhook`
- `internal`
- `with_errors`

What they mean:
- **connected**: connectors currently available
- **disabled**: manually or operationally disabled connectors
- **webhook**: count of webhook connectors
- **internal**: count of internal transport connectors
- **with_errors**: connectors with stored last error information

### E. Audit summary
Fields:
- `total_24h`
- `maintenance_24h`
- `ops_actions_24h`
- `runtime_events_24h`

What they mean:
- **total_24h**: total structured audit rows in last 24h
- **maintenance_24h**: number of maintenance sweeps in last 24h
- **ops_actions_24h**: operator-triggered ops actions in last 24h
- **runtime_events_24h**: agent/runtime-originated audit events in last 24h

---

## 7. Quick Summary Cards

The smaller cards below the summary are intentionally narrower and more tactical.

Current cards:
- **Failed / Dead Events**
- **Retry Scheduled**
- **Connected Connectors**

Purpose:
- give fast attention cues
- make obvious whether delivery health is deteriorating
- help an operator decide whether to open events or connectors first

These cards are derived from currently loaded event/connector lists, not from a separate metrics store.

---

## 8. Event Delivery Table

This section is the primary debugging surface for delivery problems.

Endpoint used:

```http
GET /api/ops/agent-events
```

### Supported filters
- `status`
- `agent_id`
- `event_type`
- `board_id`

### What each row shows
- event ID
- agent ID
- event type
- delivery status
- attempt count
- next retry time
- last response status

### When to look here
Use Event Delivery when:
- tasks are not reaching runtimes
- review requests are not arriving
- delivery is repeatedly failing
- retries look stuck
- you suspect dead-letter buildup

### Delivery statuses and meaning
- `pending` → waiting to be delivered
- `processing` → being delivered right now
- `failed` → failed, but may retry
- `dead` → retry policy exhausted
- `sent` → successful send
- `routed` → routed into internal runtime pull flow
- `ignored` → intentionally not delivered further

### Important event fields
- `delivery_attempts`: how many attempts have been made
- `next_attempt_at`: next retry schedule
- `response_status`: machine-readable outcome hint
- `response_body`: raw outcome payload or error body

---

## 9. Event Detail Drawer

When an event is opened, the drawer helps inspect deeper delivery state.

### Available details
- agent
- event type
- task ID
- board ID
- delivery status
- attempts
- created / last delivery / next retry / processed timestamps
- response status
- payload JSON
- response body

### Why this matters
This is where you confirm whether a failure is caused by:
- runtime auth issue
- wrong connector route
- temporary downstream failure
- malformed payload
- dispatcher timeout / backpressure

---

## 10. Event Actions

Ops provides two manual recovery actions for events.

### A. Retry now
Endpoint:

```http
POST /api/ops/agent-events/:id/retry
```

Used when an event is in:
- `failed`
- `processing`

Behavior:
- sets event back to pending-ish retry path
- resets retry timing to immediate eligibility
- preserves the idea that this is still the same event lifecycle

Use when:
- the error looks transient
- runtime was briefly unavailable
- connector recovered and you want another attempt now

### B. Requeue
Endpoint:

```http
POST /api/ops/agent-events/:id/requeue
```

Used when event is in:
- `dead`
- `failed`
- `ignored`
- `routed`

Behavior:
- resets event delivery state more aggressively
- zeroes delivery attempts
- places it back into pending queue

Use when:
- you intentionally want a fresh re-run of the delivery path
- previous delivery history is no longer representative
- a connector or routing configuration changed and you want to try again cleanly

### Retry vs Requeue
- **Retry** = continue this delivery lifecycle
- **Requeue** = restart it in a more forceful way

---

## 11. Connectors Table

This section helps you inspect the path MoziBoard uses to reach runtimes.

Endpoint used:

```http
GET /api/ops/connectors
```

### Supported filters
- `status`
- `agent_id`
- `connector_type`
- `transport_mode`

### What each row shows
- connector ID
- agent ID
- connector type
- transport mode
- auth type
- connector status
- endpoint URL / agent ref / session key

### When to look here
Use Connectors when:
- runtime is not receiving work
- some agents never get events
- internal bridge seems broken
- webhook connector looks registered but non-functional
- auth/config mismatch is suspected

---

## 12. Connector Detail Drawer

The connector drawer gives identity and routing context.

### Available details
- agent
- connector type
- transport mode
- auth type
- status
- endpoint URL
- base URL
- agent ref
- session key
- last success
- updated at
- last error
- metadata JSON

### Why this matters
This is the fastest way to inspect if a connector is:
- misconfigured
- stale
- disabled
- pointing to the wrong target
- missing success activity

---

## 13. Connector Actions

Two manual actions currently exist.

### A. Enable connector
Endpoint:

```http
POST /api/ops/connectors/:id/enable
```

Behavior:
- sets connector status to `connected`
- writes audit log entry

Use when:
- connector was intentionally disabled
- issue was fixed and connector should rejoin routing

### B. Disable connector
Endpoint:

```http
POST /api/ops/connectors/:id/disable
```

Behavior:
- sets connector status to `disabled`
- writes audit log entry

Use when:
- connector is noisy / broken
- you want to isolate bad delivery target
- you want to stop new work from being routed there temporarily

---

## 14. Maintenance Sweep

This is one of the most important operational controls.

Endpoint:

```http
POST /api/ops/maintenance/run
```

UI action:
- **Run maintenance** button in Ops page header

### What maintenance currently does
The current maintenance sweep performs three reconciliation jobs:

#### A. Mark stale agents offline
If an agent heartbeat is older than the stale threshold, MoziBoard marks it offline.

Purpose:
- prevent ghost-online state
- make agent presence more trustworthy

#### B. Close stale runs
If runs remain active too long without proper completion, MoziBoard closes them as failed.

Purpose:
- avoid forever-running zombie execution state
- reduce run/task confusion during incident recovery

#### C. Repair task/run drift
If a task is already `done` but the linked run is still active, maintenance reconciles the run into a done state.

Purpose:
- restore consistency between tasks and runs
- clean up state drift after partial failures or missed updates

### Maintenance output
Current response shape:

```json
{
  "ok": true,
  "report": {
    "stale_agents_marked": 0,
    "stale_runs_closed": 0,
    "drifted_tasks_repaired": 0
  }
}
```

### When to run maintenance manually
Use **Run maintenance** when:
- agent states look wrong after disconnect/crash
- runs are obviously stuck
- tasks and runs look inconsistent
- you want immediate cleanup now, not at next scheduler tick
- you are debugging a production issue and want a clean post-reconcile state

### Scheduled vs manual maintenance
There are two ways maintenance runs:

#### Scheduled
Configured via env:

```env
OPS_MAINTENANCE_EVERY=10m
```

Meaning:
- backend runs maintenance automatically every interval
- this is the normal hygiene layer

#### Manual
Triggered from Ops UI or endpoint.

Meaning:
- operator forces reconciliation immediately
- useful during incidents and debugging

### Mental model
- **scheduled maintenance** = autopilot
- **Run maintenance** = do it now

---

## 15. Audit Logging in Ops

Ops actions intentionally create structured audit traces.

Examples of logged actions:
- `retry_requested`
- `requeue_requested`
- `connector_enabled`
- `connector_disabled`
- `maintenance_sweep_ran`

### Why audit logs matter
They let you reconstruct:
- who triggered the action
- what entity was changed
- what the resulting state was
- whether the change was user-driven or system-driven

This is important for:
- debugging
- incident review
- future admin UI
- policy and security hardening

---

## 16. Recommended Operational Workflow

When something feels wrong in runtime delivery, use this order:

### Scenario A — agent not receiving work
1. open **Connectors**
2. confirm connector is `connected`
3. inspect endpoint/session/ref
4. inspect `last_error`
5. check **Event Delivery** for failed/pending events
6. retry or requeue if appropriate

### Scenario B — work seems stuck forever
1. open **Ops Summary**
2. inspect stale/failed run counts
3. inspect event retry backlog
4. run **maintenance**
5. re-check summary
6. open task/run-specific evidence if needed

### Scenario C — data looks inconsistent
Example:
- task is done but run looks active
- agent looks online but hasn’t heartbeat in ages

Steps:
1. run **maintenance**
2. refresh Ops page
3. confirm counters improved
4. inspect audit logs if you need trace proof

### Scenario D — retry backlog rising
1. inspect `retry_due`
2. filter events by `failed`
3. inspect response statuses
4. verify connector health
5. decide between retry vs requeue

---

## 17. Safety Guidelines

Ops actions are powerful enough to affect operational truth, so use them with care.

### Safe usage principles
- use **Retry** for transient failure recovery
- use **Requeue** only when you really want to restart delivery lifecycle
- use **Disable connector** to isolate damage, not as a permanent fix
- use **Run maintenance** for reconciliation, not as a substitute for root-cause fixing

### Important caution
Maintenance helps clean state, but it does **not** magically fix the underlying cause.

For example:
- if webhook auth is wrong, maintenance will not fix credentials
- if runtime is offline, maintenance only reflects the truth sooner
- if routing config is broken, requeue may just fail again

---

## 18. Current Limitations

Current Ops feature is already useful, but not finished.

### Known limitations
- ops summary is still global, not truly board-scoped
- no per-endpoint latency instrumentation yet
- no Prometheus-style metrics endpoint yet
- no pagination on ops lists yet
- permission model is not yet deeply granular per ops action
- severity semantics in UI are still lightweight
- maintenance scope is still focused on three core reconciliation jobs

---

## 19. Suggested Next Improvements

Recommended next steps for Ops maturity:

### Short-term
- board-scoped ops summary
- richer severity badges / alert coloring
- “only unhealthy” quick filters
- audit log tab in Ops UI
- small explanations/tooltips for operator actions

### Mid-term
- request latency / duration metrics
- readiness contract (`/api/readiness`)
- pagination / cursor strategy for ops tables
- dead-letter inspection improvements
- stronger permission gating for ops actions

### Longer-term
- richer signal model for agent operations center
- machine-readable operational schema
- historical charts / trendlines
- explicit incident/debug workflow playbooks

---

## 20. Quick Reference

### Main endpoints
```http
GET  /api/ops/summary
GET  /api/ops/agent-events
POST /api/ops/agent-events/:id/retry
POST /api/ops/agent-events/:id/requeue
GET  /api/ops/connectors
POST /api/ops/connectors/:id/enable
POST /api/ops/connectors/:id/disable
GET  /api/ops/audit-logs
POST /api/ops/maintenance/run
```

### Main UI actions
- Refresh
- Run maintenance
- Retry event
- Requeue event
- Enable connector
- Disable connector

### Most important operator questions
- Apakah ada backlog retry?
- Apakah connector masih sehat?
- Apakah agent benar-benar online?
- Apakah run ada yang stale?
- Apakah task/run drift perlu direconcile?

---

## 21. Related Docs

- `docs/runtime-auth-and-ops-api.md`
- `docs/backend-improvement-roadmap-2026-03.md`

If `runtime-auth-and-ops-api.md` explains the **contract**, this guide explains the **operator experience and intended usage**.
