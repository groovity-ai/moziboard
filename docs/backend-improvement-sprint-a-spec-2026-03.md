# MoziBoard Backend Improvement Sprint A Spec — March 2026

## Objective

Sprint A fokus untuk menguatkan **workflow integrity** di backend MoziBoard supaya:
- lifecycle task explicit dan predictable
- review phase punya semantics yang jelas
- state antara `tasks`, `agent_runs`, dan `agents` tidak gampang drift
- perubahan penting mulai bisa diaudit minimal

Ini adalah implementasi teknis dari roadmap backend improvement fase paling prioritas.

---

# Scope Sprint A

## In Scope
1. **Formal task state machine**
2. **Review decision model**
3. **Task / agent_runs / agents consistency guard**
4. **Minimal audit trail untuk mutation penting**

## Out of Scope
- metrics / Prometheus / perf instrumentation penuh
- `EXPLAIN ANALYZE` pass
- pagination redesign
- granular permission matrix penuh
- A2A / inter-agent delegation
- maintenance cron/worker penuh

---

# Current Baseline (Observed)

Dari code yang ada sekarang, beberapa hal penting sudah terlihat:

## Existing behavior today
- `tasks/service.go`
  - `Create()` langsung normalize status dari `list_id`
  - `Update()` masih mengizinkan patch status/list tanpa transition guard formal
  - assignment side effect langsung bikin notification + agent run queued
- `runtime/service.go`
  - `RuntimeTaskAck()` langsung set task ke `in_progress`
  - `RuntimeTaskUpdate()` langsung menerima `req.Status` hampir tanpa transition validation
  - `RuntimeTaskReviewRequest()` langsung set task ke `review`
- `runtime/repository.go`
  - `UpdateTaskAndRunState()` update `tasks` dan `agent_runs`, tapi tanpa invariant/reconciliation formal
  - `CloseOtherRuns()` sudah ada, jadi fondasi one-active-run sudah sebagian tersedia
- `tasks/helpers.go`
  - mapping status/list masih simpel, bahkan masih mengenal `backlog`

## Main weakness now
- transisi task masih implicit dan tersebar
- review belum punya action semantics terpisah
- task status / run status / agent status bisa berubah tanpa satu source of truth yang konsisten
- blocked reason / review note belum diperlakukan sebagai first-class transition metadata

---

# Desired End State After Sprint A

Setelah Sprint A selesai:

1. semua perubahan state task melewati satu **workflow policy layer**
2. runtime endpoint tidak boleh lagi seenaknya set status illegal
3. review jadi explicit lewat action semantics (`approve`, `request_changes`, `reopen`)
4. ada helper konsisten untuk sync:
   - task state
   - active run state
   - agent state
5. perubahan penting tercatat minimal sebagai audit/activity event yang lebih structured

---

# Canonical Task Workflow

## Canonical Task States
State canonical yang dipakai backend:
- `todo`
- `assigned`
- `in_progress`
- `review`
- `blocked`
- `done`

## Deferred States
Jangan diaktifkan di Sprint A dulu, tapi sisakan ruang:
- `cancelled`
- `archived`

## Legacy Mapping
Status/list lama yang masih perlu dinormalisasi:
- `doing` -> `in_progress`
- `qa` -> `review`
- `backlog` -> `todo` (untuk sekarang jangan diperlakukan state terpisah)

---

# Transition Matrix

## Allowed transitions

### From `todo`
- `todo -> assigned`
- `todo -> in_progress`
- `todo -> blocked`
- `todo -> done` **disallow** in Sprint A by default

### From `assigned`
- `assigned -> in_progress`
- `assigned -> blocked`
- `assigned -> todo` (optional, allow only if assignee removed)
- `assigned -> done` disallow

### From `in_progress`
- `in_progress -> review`
- `in_progress -> blocked`
- `in_progress -> done` allow

### From `review`
- `review -> done` via `approve`
- `review -> in_progress` via `request_changes` or `reopen`
- `review -> blocked` allow with reason

### From `blocked`
- `blocked -> in_progress`
- `blocked -> review`
- `blocked -> todo` allow only if explicitly reopened by human

### From `done`
- `done -> in_progress` only via explicit `reopen`
- all other transitions disallow by default

## Transition rules by semantics
- `blocked` requires non-empty reason
- `review -> done` should carry review note optionally
- `review -> in_progress` should carry change-request / reopen note optionally
- direct `todo -> review` disallow
- direct `assigned -> review` disallow
- direct `blocked -> done` disallow

---

# Review Decision Model

## Problem
Saat ini `review` masih cuma state. Belum ada action semantics.

## Decision
Sprint A memperkenalkan **review action contract** yang explicit.

## Review Actions
- `approve`
- `request_changes`
- `reopen`

## Semantics
- `approve`
  - valid only when current task state = `review`
  - target state = `done`
- `request_changes`
  - valid only when current task state = `review`
  - target state = `in_progress`
- `reopen`
  - valid when current task state = `review` or `done` or `blocked`
  - target state = `in_progress`

## Metadata to capture
For each review action:
- actor id
- actor type (`user` / `agent` / `system`)
- note / summary
- from_state
- to_state
- timestamp

## Persistence choice for Sprint A
**Do not create full `task_reviews` table yet** unless implementation reveals a real need.
Sprint A cukup pakai:
- existing `activities` + richer action naming, or
- small `audit_logs` table if cheap to add

Recommended action names:
- `review_requested`
- `review_approved`
- `review_changes_requested`
- `task_reopened`
- `task_blocked`
- `task_unblocked`
- `task_status_changed`
- `task_assigned`
- `task_unassigned`

---

# Consistency Guard Model

## Goal
Jadikan `tasks`, `agent_runs`, `agents` sinkron berdasarkan invariant yang jelas.

## Canonical mapping

### Task state -> run state
- `todo` -> no active run preferred
- `assigned` -> `queued`
- `in_progress` -> `running`
- `review` -> `review`
- `blocked` -> `blocked`
- `done` -> `done` and run should be closed

### Task state -> preferred agent state
If task has active assignee/agent context:
- `assigned` -> `busy` or `queued`-ish, but current system should use `busy` only after ack
- `in_progress` -> `busy`
- `review` -> `online` with `Waiting for review`
- `blocked` -> `blocked`
- `done` -> `online` and clear current_task/current_run if matching

### Run invariants
- max **1 active run** per `(task_id, agent_id)`
- active statuses are:
  - `queued`
  - `running`
  - `in_progress`
  - `blocked`
  - `review`
- terminal statuses:
  - `done`
  - `failed` (future)
  - `cancelled` (future)

## Drift scenarios to guard
1. task `done`, run still `running`
2. task `review`, agent still `busy` with stale activity
3. multiple active runs same task-agent
4. task unassigned but agent still points to `current_task_id`
5. blocked task without blocked_reason

---

# Recommended Architecture Change

## New internal module/package
Tambahkan package baru:

`backend/internal/workflow/`

### Files
- `task_workflow.go`
- `task_workflow_test.go`
- `transition.go`
- `review.go`
- `consistency.go`

Kalau mau tetap align dengan existing module pattern, bisa juga:
- `backend/internal/modules/workflow/`

Tapi secara konsep, ini lebih cocok jadi **shared domain policy package**, bukan module endpoint.

---

# Workflow Package Design

## Core types

### `TaskState`
Type alias / const untuk semua canonical task states.

### `TransitionRequest`
Suggested shape:

```go
type TransitionRequest struct {
    FromState   string
    ToState     string
    Trigger     string // manual_update, runtime_ack, runtime_update, review_action, assignment_change, system_reconcile
    ActorType   string // user, agent, system
    ActorID     string
    Note        string
    Reason      string
    AssigneeID  *string
    ReviewAction string // approve, request_changes, reopen
}
```

### `TransitionResult`
```go
type TransitionResult struct {
    Allowed          bool
    NormalizedToState string
    RequiresReason   bool
    ActivityAction   string
    ActivityDetails  string
}
```

## Core functions

### `NormalizeTaskState(raw string) string`
Responsibilities:
- map `doing` -> `in_progress`
- map `qa` -> `review`
- map `backlog` -> `todo`
- default unknown -> `todo`

### `ValidateTransition(req TransitionRequest) error`
Responsibilities:
- enforce transition matrix
- enforce reason rules
- enforce review action semantics

### `ResolveAssignmentState(oldTask, newTask) string`
Responsibilities:
- if assignee added and previous state `todo`, can normalize to `assigned`
- if assignee removed from `assigned`, can normalize back to `todo`
- avoid silently overriding explicit stronger states like `in_progress`

### `MapTaskStateToRunState(taskState string) string`
Responsibilities:
- canonical mapping for agent run status

### `DeriveAgentState(taskState string) (status string, activity string, clearCurrent bool)`
Responsibilities:
- decide preferred agent status/activity for sync helper

### `ShouldCloseRun(taskState string) bool`
Responsibilities:
- `done` closes run
- others do not

---

# Service Refactor Plan

## 1. Refactor `tasks/helpers.go`
Current helper terlalu simpel.

### Replace/upgrade with:
- canonical normalization helper
- list/status mapping only as view-layer bridge

### New rules
- UI list id remains:
  - `todo`
  - `doing`
  - `qa`
  - `blocked`
  - `done`
- backend canonical status remains separate:
  - `todo`
  - `assigned`
  - `in_progress`
  - `review`
  - `blocked`
  - `done`

### Important note
`assigned` mungkin tetap map ke `todo` list di UI kalau belum ada lane khusus.
Artinya:
- **list is presentation concern**
- **status is workflow concern**

Suggested mapping:
- `todo` -> `todo`
- `assigned` -> `todo`
- `in_progress` -> `doing`
- `review` -> `qa`
- `blocked` -> `blocked`
- `done` -> `done`

---

## 2. Refactor `tasks/service.go`

### `Create()`
Current behavior:
- status langsung dari list
- kalau ada assignee, belum otomatis pakai canonical `assigned`

### New desired behavior
On create:
1. normalize title/board/list
2. derive initial canonical status:
   - if explicit status supplied -> normalize + validate as initial state
   - else if assignee exists -> `assigned`
   - else -> `todo`
3. derive list id from canonical status
4. if assigned:
   - create queued run
   - enqueue assignment event
   - log assignment audit/activity

### Rules
- create with `blocked` allowed only if blocked_reason provided
- create with `review` or `done` should be disallowed by default in Sprint A unless clear use case exists

### `Update()`
This is the biggest refactor target.

#### New flow
1. fetch old task
2. merge patch to candidate task
3. normalize candidate status
4. derive assignment-driven implicit status adjustments:
   - no assignee -> maybe `todo`
   - new assignee on `todo` -> maybe `assigned`
5. validate transition through workflow package
6. persist task
7. run side effects based on semantic transition
8. sync run/agent consistency if relevant
9. log structured activity/audit

#### Semantic branches to handle
- status changed
- assignee changed
- blocked reason changed
- review entered
- review exited
- task reopened
- task marked done

#### New restrictions
- cannot move to `review` unless current state `in_progress` or `blocked`
- cannot mark `done` from `todo` or `assigned`
- if moving from `blocked` to non-blocked, optionally clear `blocked_reason`

---

## 3. Refactor `runtime/service.go`

Runtime path harus jadi consumer dari workflow policy, bukan bypass layer.

### `RuntimeTaskAck()`
Current:
- always sets `in_progress`

New behavior:
1. validate current task can transition to `in_progress`
2. ensure active run exists
3. sync task + run + agent using shared workflow reconciliation helper
4. log activity `acknowledged`

Expected valid source states:
- `assigned`
- `todo` (allow optionally for old tasks assigned late)
- maybe `blocked` only if explicit unblocked path, otherwise disallow

### `RuntimeTaskUpdate()`
Current:
- accepts arbitrary `req.Status`

New behavior:
1. if status empty -> treat as state-preserving progress update, not implicit transition unless needed
2. if status present -> validate via workflow
3. if `blocked` -> require non-empty `blocked_reason`
4. if `done` -> close run and clear/normalize agent state
5. if `review` -> prefer using review-request semantics instead of arbitrary update

### `RuntimeTaskReviewRequest()`
Current:
- directly sets review

New behavior:
1. validate `in_progress -> review`
2. sync run to `review`
3. sync agent to `online / Waiting for review`
4. log `review_requested`

### New endpoint/service action recommended
Add explicit service method for human review decision:
- `ApplyReviewAction(ctx, actor, taskID, action, note)`

This can live in:
- `tasks/service.go`, or
- new `workflow service` wrapper

---

# Reconciliation Helper Design

## New shared helper
Tambahkan helper yang jadi satu pintu sync:

### `ApplyTaskStateTransition(...)`
Suggested responsibilities:
- validate transition
- compute list_id from target task state
- compute target run state
- compute target agent state
- update task
- update run(s)
- close extra runs if needed
- update agent current state if assignee/agent known
- return semantic result for logging

Suggested shape:

```go
type ApplyTransitionInput struct {
    TaskID          int
    AgentID         *string
    FromState       string
    ToState         string
    ActorType       string
    ActorID         string
    Trigger         string
    Note            string
    Reason          string
    CurrentActivity string
    ResultSummary   string
}
```

This helper may initially live in `runtime/service.go` + `tasks/service.go` shared private code, but target architecture should move it to shared domain helper.

---

# Repository Changes

## `tasks/repository.go`
Recommended additions:
- `UpdateStatusFields(...)`
- `UpdateAssignment(...)`
- optional `ClearBlockedReason(...)`

Goal:
- avoid giant blind overwrite for every patch
- make mutation intent clearer

## `runtime/repository.go`
Current `UpdateTaskAndRunState()` terlalu loose.

### Replace or complement with more explicit methods:
- `UpdateTaskWorkflowState(...)`
- `UpdateActiveRunState(...)`
- `CloseActiveRuns(...)`
- `UpdateAgentWorkflowState(...)`
- `ClearAgentCurrentWorkIfMatches(...)`

If keeping existing function for speed, at minimum:
- make it return error if task update fails
- stop ignoring DB errors silently
- wrap task/run updates in transaction

## Critical requirement
**Transaction boundary** should exist for workflow mutations that touch multiple tables:
- tasks
- agent_runs
- agents
- optional audit/activity insert

Without transaction, Sprint A only fixes semantics half-way.

---

# Audit / Activity Minimal Spec

## Goal
Belum perlu full audit platform, tapi mutation penting harus tercatat konsisten.

## Option A — Reuse `activities` only
Pros:
- cheap
- no migration maybe

Cons:
- less structured
- harder to reconstruct later

## Option B — Add minimal `audit_logs` table` (recommended if cheap)
Suggested schema:

```sql
CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  action TEXT NOT NULL,
  old_value_json JSONB,
  new_value_json JSONB,
  metadata_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Minimum actions to log in Sprint A
- task status changed
- task assigned/unassigned
- review requested
- review approved
- review changes requested
- task reopened
- task blocked/unblocked
- run auto-reconciled / duplicate run closed

If table addition feels too much for this sprint:
- log into `activities` now
- but design code behind an `AuditLogger` interface so later migration is painless

---

# API / Handler Changes

## Existing task update API
Tetap bisa dipakai, tapi backend semantics diperketat.

## New explicit review action endpoint (recommended)
Add something like:
- `POST /api/tasks/:id/review-action`

Request:
```json
{
  "action": "approve",
  "note": "Looks good, ship it"
}
```

Allowed actions:
- `approve`
- `request_changes`
- `reopen`

Why add explicit endpoint?
- lebih jelas secara UX dan backend semantics
- tidak bikin `PATCH /tasks/:id` jadi overloaded
- memudahkan audit trail

## Runtime endpoints
Tetap dipakai, tapi behavior berubah:
- `/runtime/task/ack`
- `/runtime/task/update`
- `/runtime/task/review-request`

Semua wajib lewat transition validation.

---

# Data Migration Notes

## No mandatory schema explosion for Sprint A
Minimal viable path:
- reuse current schema
- maybe add `audit_logs`
- maybe add partial unique index later if compatible

## Good optional DB hardening
If schema/statuses sudah cukup stabil, pertimbangkan:

### Partial unique index for active runs
```sql
CREATE UNIQUE INDEX IF NOT EXISTS agent_runs_one_active_per_task_agent_idx
ON agent_runs(task_id, agent_id)
WHERE status IN ('queued','running','in_progress','blocked','review');
```

Cek dulu compatibility dengan data existing sebelum apply.

---

# Test Plan

## Unit tests — workflow package
Minimum cases:

### normalization
- `doing -> in_progress`
- `qa -> review`
- `backlog -> todo`
- unknown -> `todo`

### valid transitions
- `todo -> assigned`
- `assigned -> in_progress`
- `in_progress -> review`
- `review -> done` via approve
- `review -> in_progress` via request_changes
- `done -> in_progress` via reopen

### invalid transitions
- `todo -> review`
- `assigned -> done`
- `blocked -> done`
- `review -> done` without approve semantics if explicit action path required
- `blocked` without reason

## Service tests

### tasks service
- create with assignee => status becomes `assigned`
- patch assignee on todo => status becomes `assigned`
- unassign assigned task => status back to `todo`
- patch invalid status transition => error

### runtime service
- ack from assigned => ok
- update with invalid transition => error
- blocked without reason => error
- review request from in_progress => ok
- review request from todo => error

### consistency tests
- duplicate active runs => extras closed
- done task => run closed, agent cleared/online
- review => agent waiting_review semantics applied

---

# Suggested Implementation Order

## Step 1 — Domain policy first
- create `internal/workflow/` package
- implement constants, normalization, transition validator, mapping helpers
- write tests first

## Step 2 — Task service integration
- refactor `tasks/service.go` create/update to use workflow package
- keep changes local before touching runtime

## Step 3 — Runtime integration
- refactor runtime ack/update/review-request to use same transition rules
- stop accepting arbitrary status mutation

## Step 4 — Reconciliation helper
- centralize task/run/agent sync logic
- add transaction support where needed

## Step 5 — Minimal audit trail
- implement `AuditLogger` interface
- either back it by `activities` or new `audit_logs`

## Step 6 — Validation & smoke
- Docker build
- restart backend
- smoke test these flows:
  1. create assigned task
  2. runtime ack
  3. runtime update progress
  4. runtime request review
  5. human approve review
  6. blocked -> resume path

---

# Definition of Done

Sprint A dianggap selesai jika:

1. task state transitions sudah enforced centrally
2. runtime endpoints tidak bisa lagi bypass lifecycle rules
3. review punya explicit action semantics
4. duplicate active runs ditutup konsisten
5. done/review/blocked state menghasilkan sync yang benar di task/run/agent
6. ada jejak audit/activity minimal untuk perubahan penting
7. backend build + restart + smoke test lulus

---

# Concrete File Touch List

## Likely touched
- `backend/internal/modules/tasks/helpers.go`
- `backend/internal/modules/tasks/service.go`
- `backend/internal/modules/tasks/repository.go`
- `backend/internal/modules/runtime/service.go`
- `backend/internal/modules/runtime/repository.go`
- `backend/internal/modules/tasks/handler.go` (if review action endpoint added)
- `backend/internal/modules/runtime/handler.go` (validation changes only)
- `backend/internal/modules/agents/service.go` (maybe small sync tweaks)
- `backend/internal/modules/agentinfra/service.go` (if run helper touched)
- `backend/internal/modules/dispatch/service.go` (only if event side effects need adjustment)

## New files recommended
- `backend/internal/workflow/task_workflow.go`
- `backend/internal/workflow/task_workflow_test.go`
- `backend/internal/workflow/review.go`
- `backend/internal/workflow/consistency.go`
- optional migration file for `audit_logs`
- optional `docs/sprint-a-smoke-checklist.md`

---

# Final Recommendation

Jangan mulai Sprint A dari DB dulu.
Mulai dari **policy code + tests**, karena ini sumber kebenaran barunya.

Urutan paling sehat:
1. state model
2. transition validator
3. service integration
4. transactional reconciliation
5. audit trail
6. smoke

Kalau ini beres, Sprint B dan C bakal jauh lebih gampang karena backend semantics-nya sudah dewasa.
