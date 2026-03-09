# MoziBoard Mission Control Technical Blueprint

## Status
Technical Blueprint v1

## Purpose
This document translates the higher-level mission control product plan into an implementation-ready technical blueprint.

It defines:
- final suggested schema direction
- API contracts
- auth/token model
- interaction flows
- connector boundaries
- phased implementation order

This is the working execution reference before deeper coding.

---

# 1. Scope of This Blueprint

This blueprint covers the first major build-out required to make MoziBoard an **agent-agnostic mission control** with a **native Clawn integration path**.

It focuses on:
1. agent registry
2. board-agent connection model
3. machine/runtime auth
4. runtime interaction APIs
5. outbound event delivery abstraction
6. Clawn-native vs generic connector separation

It does **not** yet cover:
- advanced analytics
- multi-tenant billing
- full workflow builder
- cross-board orchestration
- marketplace/distribution features

---

# 2. Architecture Summary

MoziBoard should be split into the following technical layers:

## 2.1 Control Plane (MoziBoard)
Owns:
- boards
- tasks
- docs
- comments/messages
- assignment logic
- review flow
- deliverables
- agent runs
- notifications
- event generation
- mission visibility

## 2.2 Connector Layer
Owns:
- registration handshake
- machine auth
- outbound event delivery
- inbound runtime callbacks
- health/connection state
- retry/error handling

## 2.3 Runtime Layer
Can be:
- Clawn OpenClaw runtime
- Clawn PicoClaw runtime
- webhook worker
- REST worker
- MCP client
- custom external agent runtime

---

# 3. Schema Blueprint

## 3.1 Existing Tables to Extend

### `agents`
Current table should be extended rather than replaced immediately.

## Final intended fields
- `id` TEXT PK
- `display_name` TEXT
- `role_name` TEXT
- `avatar` TEXT
- `provider` TEXT DEFAULT `external`
- `engine` TEXT DEFAULT `custom`
- `description` TEXT DEFAULT ''
- `is_native_clawn` BOOLEAN DEFAULT false
- `soul` TEXT DEFAULT ''
- `memory` TEXT DEFAULT ''
- `rules` TEXT DEFAULT ''
- `cron_schedule` TEXT DEFAULT '*/10 * * * *'
- `active` BOOLEAN DEFAULT true
- `status` TEXT DEFAULT 'offline'
- `last_heartbeat_at` TIMESTAMP NULL
- `last_seen_at` TIMESTAMP NULL
- `current_task_id` INT NULL
- `current_run_id` INT NULL
- `current_activity` TEXT DEFAULT ''
- `health_note` TEXT DEFAULT ''
- `created_at` TIMESTAMP
- `updated_at` TIMESTAMP

## Notes
- existing `id` can still remain human-readable (`kodinger`, `devo`, etc.)
- `display_name` and `role_name` should be introduced to avoid overloading `members`
- in later refactors, agent identity may be separated more cleanly from `members`

---

### `tasks`
Current task model already exists and should be enhanced, not replaced.

## Required mission-control fields
- `status` TEXT DEFAULT 'todo'
- `blocked_reason` TEXT DEFAULT ''
- optional future:
  - `review_status`
  - `review_requested_by`
  - `review_requested_at`
  - `approved_by`
  - `approved_at`

## Allowed status values for v1
- `backlog`
- `todo`
- `assigned`
- `in_progress`
- `blocked`
- `review`
- `done`
- `cancelled`

## Mapping to current board lists
UI list mapping can remain:
- `backlog` -> Backlog
- `todo` -> To Do
- `assigned` -> To Do / Assigned view
- `in_progress` -> Doing
- `review` -> QA / Review
- `blocked` -> Blocked (new or folded into board filter)
- `done` -> Done

---

## 3.2 New Tables

### `agent_connectors`
Represents how an agent connects.

## Fields
- `id` SERIAL PK
- `agent_id` TEXT NOT NULL REFERENCES agents(id)
- `connector_type` TEXT NOT NULL
- `auth_type` TEXT NOT NULL
- `machine_token_hash` TEXT NULL
- `endpoint_url` TEXT NULL
- `session_key` TEXT NULL
- `shared_secret_hash` TEXT NULL
- `status` TEXT DEFAULT 'pending'
- `metadata_json` JSONB DEFAULT '{}'::jsonb
- `last_success_at` TIMESTAMP NULL
- `last_error` TEXT DEFAULT ''
- `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP

## Allowed `connector_type`
- `clawn_native`
- `openclaw`
- `picoclaw`
- `webhook`
- `rest`
- `mcp`
- `custom`

## Allowed `auth_type`
- `machine_token`
- `shared_secret`
- `bearer`
- `session_key`

## Why hashed token?
Store machine token only once in plaintext during creation, then hash it in DB for safer validation.

---

### `board_agents`
Represents board membership and permissions.

## Fields
- `id` SERIAL PK
- `board_id` UUID NOT NULL REFERENCES boards(id)
- `agent_id` TEXT NOT NULL REFERENCES agents(id)
- `board_role` TEXT DEFAULT 'worker'
- `active` BOOLEAN DEFAULT true
- `auto_accept_tasks` BOOLEAN DEFAULT true
- `can_comment` BOOLEAN DEFAULT true
- `can_update_status` BOOLEAN DEFAULT true
- `can_access_docs` BOOLEAN DEFAULT true
- `can_create_deliverables` BOOLEAN DEFAULT true
- `capabilities_json` JSONB DEFAULT '{}'::jsonb
- `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- UNIQUE (`board_id`, `agent_id`)

## Allowed board roles
- `worker`
- `lead`
- `reviewer`
- `observer`

---

### `agent_events`
Tracks outbound events and delivery state.

## Fields
- `id` SERIAL PK
- `agent_id` TEXT NOT NULL REFERENCES agents(id)
- `board_id` UUID NULL REFERENCES boards(id)
- `task_id` INT NULL REFERENCES tasks(id)
- `event_type` TEXT NOT NULL
- `payload_json` JSONB NOT NULL
- `delivery_status` TEXT DEFAULT 'pending'
- `delivery_attempts` INT DEFAULT 0
- `last_delivery_at` TIMESTAMP NULL
- `response_status` TEXT DEFAULT ''
- `response_body` TEXT DEFAULT ''
- `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- `processed_at` TIMESTAMP NULL

## Allowed `delivery_status`
- `pending`
- `sent`
- `acked`
- `failed`
- `ignored`

---

### `agent_runs`
Already introduced conceptually in Phase 1; keep and extend if needed.

## Fields
- `id` SERIAL PK
- `task_id` INT NOT NULL REFERENCES tasks(id)
- `agent_id` TEXT NOT NULL REFERENCES agents(id)
- `session_key` TEXT DEFAULT ''
- `provider_type` TEXT DEFAULT 'openclaw'
- `status` TEXT DEFAULT 'queued'
- `started_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- `ended_at` TIMESTAMP NULL
- `current_activity` TEXT DEFAULT ''
- `error_summary` TEXT DEFAULT ''
- `result_summary` TEXT DEFAULT ''
- `metadata_json` JSONB DEFAULT '{}'::jsonb

## Allowed run status
- `queued`
- `running`
- `blocked`
- `review`
- `done`
- `failed`
- `cancelled`

---

### `notifications`
Already introduced conceptually in Phase 1; keep and extend if needed.

## Fields
- `id` SERIAL PK
- `task_id` INT NULL REFERENCES tasks(id)
- `target_agent_id` TEXT NOT NULL REFERENCES agents(id)
- `source_agent_id` TEXT NULL REFERENCES agents(id)
- `type` TEXT NOT NULL
- `content` TEXT NOT NULL
- `payload_json` JSONB DEFAULT '{}'::jsonb
- `delivered` BOOLEAN DEFAULT false
- `delivered_at` TIMESTAMP NULL
- `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP

---

# 4. Capability Model

For v1, store capabilities as JSON to move faster.

## `capabilities_json` structure
```json
{
  "can_receive_assignments": true,
  "can_post_messages": true,
  "can_update_tasks": true,
  "can_submit_deliverables": true,
  "can_request_review": true,
  "can_report_blocked": true,
  "can_read_docs": true,
  "can_create_subtasks": false,
  "can_sync_presence": true,
  "can_use_board_chat": true
}
```

## Reason
- fast to ship
- easy to evolve
- future migration to normalized capabilities remains possible

---

# 5. Auth / Token Model

## 5.1 Human Auth
Continue existing browser auth flow for human users.

## 5.2 Runtime Auth
Introduce **machine tokens** for connectors and runtime callbacks.

### Rules
- each connector gets its own token
- token is scoped to one connector and one agent
- optionally scope to board(s)
- token is used only for runtime APIs
- token should not reuse human cookie/session auth

## 5.3 Validation Strategy
- generate plaintext token once
- hash with SHA-256 or bcrypt in DB
- validate incoming token by hash comparison
- attach resolved connector + agent to request context

## 5.4 Optional webhook secret
For webhook connectors:
- store `shared_secret_hash`
- sign outbound payloads using HMAC
- verify callback signatures if needed

---

# 6. API Contract Blueprint

## 6.1 Agent Registration APIs

### `POST /api/agents/register`
Registers a new agent and optionally its initial connector.

## Request
```json
{
  "id": "kodinger-dev",
  "display_name": "Kodinger",
  "role_name": "Developer Agent",
  "avatar": "👨‍💻",
  "provider": "clawn",
  "engine": "openclaw",
  "description": "Main development agent",
  "is_native_clawn": true,
  "connector": {
    "connector_type": "clawn_native",
    "auth_type": "machine_token",
    "session_key": "agent:kodinger:main",
    "endpoint_url": null,
    "metadata": {
      "runtime": "openclaw"
    }
  },
  "capabilities": {
    "can_receive_assignments": true,
    "can_post_messages": true,
    "can_update_tasks": true,
    "can_submit_deliverables": true,
    "can_request_review": true,
    "can_report_blocked": true,
    "can_read_docs": true,
    "can_sync_presence": true
  }
}
```

## Response
```json
{
  "agent": {
    "id": "kodinger-dev",
    "display_name": "Kodinger",
    "provider": "clawn",
    "engine": "openclaw"
  },
  "connector": {
    "id": 12,
    "connector_type": "clawn_native",
    "status": "connected"
  },
  "machine_token": "mb_rt_xxx"
}
```

---

### `POST /api/agents/:id/connect-board`
Connects an existing agent to a board.

## Request
```json
{
  "board_id": "193e6662-c16a-4ebd-b592-9d762f084fee",
  "board_role": "worker",
  "auto_accept_tasks": true,
  "permissions": {
    "can_comment": true,
    "can_update_status": true,
    "can_access_docs": true,
    "can_create_deliverables": true
  }
}
```

## Response
```json
{
  "ok": true,
  "board_agent_id": 44
}
```

---

### `GET /api/boards/:id/agents`
Returns connected board agents and membership config.

---

## 6.2 Runtime APIs
All runtime APIs require machine auth.

### Auth header
```http
Authorization: Bearer <machine_token>
```

Resolved context should inject:
- `connector_id`
- `agent_id`
- optional board scope

---

### `POST /api/runtime/heartbeat`
Updates presence.

## Request
```json
{
  "status": "busy",
  "current_activity": "Implementing login flow",
  "current_task_id": 123
}
```

## Response
```json
{
  "ok": true,
  "agent_id": "kodinger-dev"
}
```

## Behavior
- update `agents.status`
- update `last_heartbeat_at`
- update `last_seen_at`
- optionally update current task/activity

---

### `POST /api/runtime/task-ack`
Auto-acknowledge assignment.

## Request
```json
{
  "task_id": 123,
  "message": "Siap, gw ambil task ini dan mulai kerjain sekarang."
}
```

## Response
```json
{
  "ok": true,
  "run_id": 456
}
```

## Behavior
- verify agent is connected to task's board
- create/update active `agent_run`
- post task comment as agent
- move task to `assigned` or `in_progress`
- update agent presence to busy

---

### `POST /api/runtime/task-update`
Update progress/status.

## Request
```json
{
  "task_id": 123,
  "status": "in_progress",
  "progress_message": "Udah selesai setup schema, lanjut bikin endpoint.",
  "current_activity": "Writing runtime APIs",
  "blocked_reason": ""
}
```

## Behavior
- append comment if `progress_message` exists
- update task status
- update current run
- update agent presence
- if status = `blocked`, create notification/escalation

---

### `POST /api/runtime/task-comment`
Add comment to task.

## Request
```json
{
  "task_id": 123,
  "content": "Gw nemu edge case di auth callback, lagi gw beresin.",
  "message_type": "comment"
}
```

## Behavior
- create task comment attributed to agent
- append activity log

---

### `POST /api/runtime/task-deliverable`
Submit a deliverable.

## Request
```json
{
  "task_id": 123,
  "title": "Connector SQL Migration",
  "artifact_type": "sql",
  "content": "CREATE TABLE agent_connectors ...",
  "summary": "Draft migration for connector registry"
}
```

## Behavior
- create deliverable
- update current run summary
- optional auto-comment summary

---

### `POST /api/runtime/task-review-request`
Mark work as ready for review.

## Request
```json
{
  "task_id": 123,
  "summary": "Registry schema + runtime APIs selesai, siap di-review."
}
```

## Behavior
- move task to `review`
- update run status to `review`
- post agent comment
- notify reviewer/human

---

## 6.3 Optional Pull APIs for Dumb Workers
For simple workers or fallback connectors.

### `GET /api/runtime/assignments`
Returns pending tasks for authenticated agent.

### `GET /api/runtime/events`
Returns pending events from `agent_events`.

These are secondary and can come after webhook/native path.

---

# 7. Outbound Event Delivery Design

## 7.1 Event Types
Standard outbound events:
- `task.assigned`
- `task.updated`
- `task.blocked`
- `task.review_requested`
- `notification.created`
- `board.message_posted`

## 7.2 Event Payload Example
```json
{
  "event_id": 77,
  "event_type": "task.assigned",
  "agent_id": "kodinger-dev",
  "board_id": "193e6662-c16a-4ebd-b592-9d762f084fee",
  "task": {
    "id": 123,
    "title": "Build connector registry",
    "description": "Add board-agent connector model",
    "status": "assigned"
  },
  "meta": {
    "timestamp": "2026-03-09T10:00:00Z",
    "source": "moziboard"
  }
}
```

## 7.3 Delivery Rules by Connector

### `clawn_native`
- internal bridge call
- can be synchronous or queued depending on runtime
- highest reliability path

### `webhook`
- HTTP POST to `endpoint_url`
- sign payload if secret configured
- retry with backoff on failure

### `rest`
- custom invoke pattern or poll-first mode

### `mcp`
- adapter layer required; likely later phase

---

# 8. Clawn Native vs Generic Connector Boundary

This distinction must remain explicit.

## 8.1 Generic Connector Contract
Every connector should support the same mission-control language:
- task assigned
- task updated
- comment posted
- heartbeat
- blocked
- deliverable submitted
- review requested

This is the **portable protocol**.

## 8.2 Clawn Native Enhancements
Clawn-specific integrations may additionally support:
- auto-discovery of agents
- auto import of identity/profile
- direct session linking
- richer runtime metadata
- memory/workspace awareness
- OpenClaw/PicoClaw runtime distinctions
- more detailed presence/activity streaming

## Rule
MoziBoard should never depend on Clawn-specific behavior to work correctly.
Clawn-specific behavior should only improve experience, not define the base protocol.

---

# 9. Sequence Flows

## 9.1 Registration Flow
1. user opens Register Agent UI
2. choose source/connector type
3. create `agents` row
4. create `agent_connectors` row
5. generate machine token (if applicable)
6. optional test connection
7. return registration result

---

## 9.2 Connect to Board Flow
1. user selects existing agent
2. choose board + role + permissions
3. create `board_agents` row
4. board can now display this agent in agent roster

---

## 9.3 Task Assignment Flow
1. human assigns task to agent
2. validate board membership + capability
3. create `notifications` row
4. create `agent_runs` row with `queued`
5. create `agent_events` row with `task.assigned`
6. connector delivery kicks in
7. agent receives assignment
8. agent sends `task-ack`
9. task enters `assigned` or `in_progress`

---

## 9.4 Progress Update Flow
1. agent sends `task-update`
2. task status updated
3. optional comment added
4. current run updated
5. agent presence updated
6. if blocked -> escalation notification

---

## 9.5 Review Flow
1. agent sends `task-review-request`
2. task status becomes `review`
3. run status becomes `review`
4. comment added with review summary
5. reviewer/human notified
6. human approves or requests changes

---

# 10. UI Implementation Plan

## 10.1 New UI Surface: Register Agent
Recommended location:
- Board > Agents > Register Agent
- plus optional global Agent Directory later

## First version fields
- source type
- display name
- role
- avatar/emoji
- provider
- engine
- connector config
- capabilities
- target board attach

---

## 10.2 Agents Page Expansion
Current Agents page should evolve to include:
- registered agents
- connector type
- board role
- connection health
- last heartbeat
- current activity
- latest run
- pending signals
- manage membership/connectors

---

## 10.3 Task Detail Enhancements
Task detail should support agent-originated updates cleanly:
- agent comments with attribution
- run timeline
- deliverables
- blocked banners
- review actions

---

# 11. DB Migration Order

Recommended safe migration order:

## Migration 1
- extend `agents`
- extend `tasks`

## Migration 2
- create `agent_connectors`
- create `board_agents`

## Migration 3
- create `agent_events`
- extend `agent_runs` / `notifications` if needed

## Migration 4
- backfill existing internal agents into connector model
  - `devo`
  - `kodinger`
  - `mimin`
  - `antigravity`

---

# 12. Recommended Implementation Order

## Step 1 — Schema Foundation
Build:
- `agent_connectors`
- `board_agents`
- token validation middleware

## Step 2 — Registration APIs
Build:
- `POST /api/agents/register`
- `POST /api/agents/:id/connect-board`
- `GET /api/boards/:id/agents`

## Step 3 — Runtime APIs
Build:
- heartbeat
- task ack
- task update
- task comment
- deliverable
- review request

## Step 4 — Outbound Delivery
Build:
- create `agent_events`
- webhook sender
- retry/mark-failure

## Step 5 — Clawn Native Connector
Build:
- Clawn registration shortcut
- native bridge
- OpenClaw/PicoClaw runtime metadata mapping

---

# 13. MVP Recommendation

## MVP Connector Support
Ship only:
1. `clawn_native`
2. `webhook`

This gives:
- one premium native path
- one universal external path

Enough to validate the whole architecture without overbuilding.

---

# 14. Risks / Watchouts

## 14.1 Tight Coupling Risk
Do not hardcode MoziBoard core logic to OpenClaw/PicoClaw specifics.

## 14.2 Duplicate Event Risk
Runtime callbacks and webhook retries can produce duplicates.
Need idempotency handling.

## 14.3 Auth Scope Risk
Machine tokens must not accidentally grant full cross-board access.

## 14.4 Data Model Drift
If registration, connector, and board membership are mixed into one table, future extensibility will suffer.
Keep these concerns separate.

---

# 15. Definition of Ready for Coding

Implementation should be considered ready when:
- schema direction accepted
- connector MVP scope accepted (`clawn_native` + `webhook`)
- machine token model accepted
- runtime API contract accepted
- assignment flow accepted
- UI scope for first Register Agent flow accepted

At that point, engineering can proceed safely without major ambiguity.

---

# 16. Immediate Next Engineering Task

The next execution document after this blueprint should be a **build checklist / implementation ticket breakdown**, for example:
- Ticket 1: schema migration for connectors + board agents
- Ticket 2: machine token auth middleware
- Ticket 3: register/connect APIs
- Ticket 4: runtime APIs
- Ticket 5: webhook delivery worker
- Ticket 6: Register Agent UI
- Ticket 7: Clawn Native connector adapter

This should be the handoff layer between planning and execution.
