# Clawn Agent Picker → MoziBoard Connect Flow

Last updated: 2026-03-10

This document defines the product and technical blueprint for a one-click **"Connect from Clawn"** flow inside MoziBoard.

The goal is to replace manual connector configuration with a guided picker that lets a user:

1. browse Clawn agents they own
2. select one or more agents that are runtime-ready
3. connect them to a MoziBoard board
4. let MoziBoard automatically create the required agent, connector, and board membership records

---

# 1. Problem Statement

Today, connecting a Clawn-backed agent to MoziBoard requires internal knowledge such as:
- `agent_ref`
- `session_key`
- connector type
- transport mode
- machine token lifecycle
- runtime auth expectations

That is acceptable for development, but it is not acceptable as the long-term product UX.

From the user's perspective, the intent is simple:

> "Show me the agents I already have in Clawn, and let me attach one to this board."

The user should not be required to understand transport-level configuration.

---

# 2. Product Goal

Turn manual runtime registration into a **source picker flow**.

Inside MoziBoard, the user should be able to:
- open **Register Agent**
- choose **Clawn** as the source
- see only Clawn agents/projects they are allowed to use
- select one agent
- choose board permissions
- click **Connect**

MoziBoard should then automatically:
- resolve the selected Clawn agent's runtime metadata
- create or update the MoziBoard `agents` record
- create or update the `agent_connectors` record
- attach the agent to the board through `board_agents`
- generate and store runtime auth material as needed

---

# 3. UX Vision

## 3.1 Entry Point

Primary entry point:
- `Board > Agents > Register Agent`

Within the Register Agent flow, the user sees source options:
- Clawn
- Manual / Custom Connector
- Future sources (OpenClaw direct, webhook, remote runtime, etc.)

For this blueprint, the focus is:
- **Clawn → Agent Picker → Connect**

---

## 3.2 User Journey

### Step 1 — Open picker
User clicks:
- `Connect Agent`
- then chooses `From Clawn`

### Step 2 — Browse eligible Clawn agents
MoziBoard shows a list of Clawn agents available to the current user.

Each item should show:
- agent name
- project name
- model
- runtime type
- readiness status
- last seen / runtime health
- already connected badge if the board already has it

### Step 3 — Select agent
User selects one agent.

### Step 4 — Configure board role
User configures board-level behavior only, not runtime internals:
- board role
- auto accept tasks
- can comment
- can update task status
- can access docs
- can create deliverables

### Step 5 — Confirm connect
User clicks `Connect`.

### Step 6 — System performs automation
MoziBoard performs all backend orchestration:
- validate ownership/access
- fetch runtime metadata from Clawn
- upsert internal agent record
- upsert connector config
- create board membership
- return success state

### Step 7 — Board reflects connected agent
User is returned to the board agent list and can immediately:
- assign work
- see agent in board membership
- inspect connector in Ops Dashboard

---

# 4. Core Principle: User Picks Intent, System Builds Runtime Config

The user should choose:
- **which agent**
- **which board**
- **what permissions**

The system should decide:
- connector type
- transport mode
- auth mode
- agent reference mapping
- runtime session linkage
- token generation and storage

This is the core product rule for the Clawn native path.

---

# 5. Data Sources and Integration Boundary

There are two ways for MoziBoard to discover Clawn agents.

## Option A — Direct internal adapter (recommended for Phase 1)
MoziBoard reads Clawn-owned data through a trusted internal adapter.

This adapter may use:
- direct database reads
- direct internal API calls
- service-to-service authenticated fetches

Use this when both systems are controlled by the same ecosystem and speed matters.

### Pros
- faster to ship
- minimal frontend complexity
- enough for internal ecosystem rollout

### Cons
- tighter coupling to Clawn schema/runtime model
- schema drift risk if Clawn changes aggressively

---

## Option B — Formal Clawn Integration API (recommended for later hardening)
MoziBoard fetches data from an explicit Clawn integration API.

Example responsibilities on Clawn side:
- list eligible agents for current user
- return runtime metadata for selected agent
- expose connection readiness state
- optionally return signed connection payloads

### Pros
- cleaner boundary
- easier long-term maintenance
- better security and ownership enforcement

### Cons
- requires more integration work upfront

---

# 6. Ownership and Access Model

This flow is only safe if MoziBoard can determine which Clawn agents belong to the current user.

## Required rule
MoziBoard must only list agents that the authenticated MoziBoard user is allowed to connect.

That means we need a trust mapping between:
- MoziBoard user identity
- Clawn user identity

## Minimum acceptable access rule
An agent can appear in the picker only if:
- the current MoziBoard user owns the Clawn project, or
- the current MoziBoard user has explicit permission in Clawn, or
- the current user is an admin with cross-project visibility

## Never do this
Do not let the frontend pass raw runtime metadata and assume it is trustworthy.

The selected `clawn_agent_id` should be treated as a lookup key only.
MoziBoard must resolve sensitive/runtime fields server-side.

---

# 7. Required Clawn Metadata

For each connectable Clawn agent, MoziBoard ultimately needs enough information to build a valid native connector.

## 7.1 Picker list payload
This is what the UI needs to render the picker.

```json
{
  "id": "clawn-agent-123",
  "display_name": "Kodinger 5.4",
  "project_id": "proj_abc",
  "project_name": "Groovity Core",
  "owner_user_id": "user_1",
  "runtime_type": "openclaw",
  "model": "openai-codex/gpt-5.4",
  "status": "online",
  "last_seen_at": "2026-03-10T02:40:00Z",
  "is_connectable": true,
  "connect_reason": "runtime_ready",
  "already_connected": false
}
```

## 7.2 Runtime resolution payload
This is what the backend needs to actually create the connector.

```json
{
  "id": "clawn-agent-123",
  "display_name": "Kodinger 5.4",
  "provider": "clawn",
  "engine": "openclaw",
  "runtime": {
    "connector_type": "clawn_native",
    "transport_mode": "internal",
    "agent_ref": "serve-codex54",
    "session_key": "sess_123",
    "base_url": "",
    "auth_mode": "machine_token"
  },
  "capabilities": {
    "can_comment": true,
    "can_update_status": true,
    "can_access_docs": true,
    "can_create_deliverables": true
  }
}
```

## Notes
- `agent_ref` and `session_key` must never be typed by the end user in this flow
- `is_connectable=false` should hide the connect action or disable it with a reason
- `runtime_type` may later support `openclaw`, `picoclaw`, `remote_http`, or `pull`

---

# 8. Proposed MoziBoard Backend API

To keep the frontend simple, MoziBoard should expose a Clawn integration facade.

## 8.1 List eligible Clawn agents
### `GET /api/integrations/clawn/agents?board_id=:id`

Returns Clawn agents available to the current user and enriched with connection state.

### Response example
```json
[
  {
    "id": "clawn-agent-123",
    "display_name": "Kodinger 5.4",
    "project_id": "proj_abc",
    "project_name": "Groovity Core",
    "runtime_type": "openclaw",
    "model": "openai-codex/gpt-5.4",
    "status": "online",
    "last_seen_at": "2026-03-10T02:40:00Z",
    "is_connectable": true,
    "connect_reason": "runtime_ready",
    "already_connected": false,
    "existing_agent_id": null
  }
]
```

### Server responsibilities
- authenticate MoziBoard user
- resolve corresponding Clawn identity
- fetch Clawn projects/agents owned by that user
- filter to agents that are runtime-ready
- annotate if already connected to current board

---

## 8.2 Connect selected Clawn agent
### `POST /api/integrations/clawn/connect`

### Request body
```json
{
  "board_id": "5dd8c641-f7af-46b1-91bd-ada0d6384fa0",
  "clawn_agent_id": "clawn-agent-123",
  "board_role": "executor",
  "auto_accept_tasks": true,
  "can_comment": true,
  "can_update_status": true,
  "can_access_docs": true,
  "can_create_deliverables": true
}
```

### Response body
```json
{
  "ok": true,
  "agent": {
    "id": "clawn:clawn-agent-123",
    "display_name": "Kodinger 5.4"
  },
  "connector": {
    "id": 42,
    "connector_type": "clawn_native",
    "transport_mode": "internal",
    "status": "connected"
  },
  "board_agent": {
    "id": 77,
    "board_id": "5dd8c641-f7af-46b1-91bd-ada0d6384fa0",
    "agent_id": "clawn:clawn-agent-123"
  }
}
```

---

# 9. Proposed Server-Side Connect Algorithm

When `POST /api/integrations/clawn/connect` is called, MoziBoard should do the following:

## Step 1 — Authenticate user
Resolve currently logged-in MoziBoard user.

## Step 2 — Resolve Clawn identity
Map the MoziBoard user to the Clawn user/account context.

## Step 3 — Validate board access
Ensure the user is allowed to modify the target board.

## Step 4 — Fetch selected agent from Clawn
Resolve the selected `clawn_agent_id` from trusted Clawn data.

## Step 5 — Validate ownership and readiness
Ensure:
- agent belongs to the current user or accessible scope
- runtime is connectable
- required runtime fields are present
- runtime type is supported

## Step 6 — Upsert MoziBoard `agents`
Create or update a normalized agent identity.

Recommended ID strategy:
- `clawn:<clawn_agent_id>`

This prevents collisions with manual agents or future external runtimes.

Fields to sync:
- `id`
- `name` / `display_name`
- `provider = clawn`
- `engine = openclaw | picoclaw | ...`
- presence/status metadata when available

## Step 7 — Upsert `agent_connectors`
Create or update one native connector using server-resolved runtime metadata.

Recommended defaults for initial Clawn integration:
- `connector_type = clawn_native`
- `transport_mode = internal`
- `auth_type = machine_token`
- `status = connected`

MoziBoard should:
- generate a machine token if one does not exist
- hash and store it in `machine_token_hash`
- store runtime metadata like `agent_ref`, `session_key`, `base_url`
- optionally store a small metadata JSON with source context

Example metadata:
```json
{
  "source": "clawn",
  "clawn_agent_id": "clawn-agent-123",
  "project_id": "proj_abc",
  "runtime_type": "openclaw"
}
```

## Step 8 — Upsert `board_agents`
Attach the agent to the selected board with the chosen permissions.

## Step 9 — Return hydrated result
Return the normalized agent + connector + board membership summary for the UI.

---

# 10. Database Mapping Strategy

## 10.1 Agent identity
MoziBoard should treat Clawn as a source of agent identity, not just connector transport.

Recommended internal mapping:
- `agents.id = clawn:<clawn_agent_id>`
- `agents.provider = clawn`
- `agents.engine = <runtime engine from Clawn>`

This makes the data model explicit and future-safe.

---

## 10.2 Connector mapping
One Clawn agent should typically map to one primary native connector.

Recommended connector uniqueness rule for MVP:
- one active primary connector per `(agent_id, connector_type, transport_mode)`

If reconnecting the same Clawn agent:
- update existing connector instead of creating duplicates when possible

---

## 10.3 Board membership mapping
`board_agents` remains board-scoped.

This means one Clawn agent can be attached to multiple boards later, subject to product rules.

For MVP, it is acceptable to allow the same agent on multiple boards if the user intentionally connects it.

---

# 11. UI Blueprint

## 11.1 Modal structure
### Modal title
`Connect Agent from Clawn`

### Section A — Source list
Search + filterable list of Clawn agents.

Each card should show:
- display name
- project name
- model
- runtime type
- status badge
- readiness badge
- already connected badge

### Section B — Board behavior form
Fields:
- board role
- auto accept tasks
- can comment
- can update task status
- can access docs
- can create deliverables

### Section C — CTA
Primary action:
- `Connect Agent`

Secondary action:
- `Cancel`

---

## 11.2 Empty states
### No Clawn agents
Show:
- `No Clawn agents found`
- `Create or start an agent in Clawn first, then come back here.`

### Agent exists but not runtime-ready
Show disabled item with reason, such as:
- no active runtime session
- runtime metadata missing
- agent offline and not connectable yet

---

## 11.3 Success state
After connect success:
- close modal
- refresh board agent list
- optionally show success toast
- optionally deep-link to Ops Dashboard for inspection

---

# 12. Security Rules

## 12.1 Frontend never submits internal runtime fields
The frontend should never post:
- `agent_ref`
- `session_key`
- `base_url`
- `machine_token`
- `shared_secret`

Those must be resolved server-side.

## 12.2 Server must verify ownership
`clawn_agent_id` is not trusted by itself.
Server must re-check ownership against authenticated user context.

## 12.3 Machine token handling
If machine tokens are used for runtime callbacks:
- generate server-side only
- store only token hash in DB
- do not expose token back to the UI unless explicitly necessary
- if Clawn runtime must receive a token, pass it through a trusted server-to-server path

## 12.4 Auditability
MoziBoard should log or record:
- who connected the agent
- which board it was connected to
- when the connector was created/updated
- what source integration was used

---

# 13. Failure Modes and UX Handling

## Failure: user no longer owns agent
Return:
- `403 forbidden`
- UI message: `This Clawn agent is no longer available to your account.`

## Failure: runtime metadata incomplete
Return:
- `409 conflict`
- UI message: `This agent exists but is not runtime-ready yet.`

## Failure: connector upsert succeeds but board attach fails
Behavior:
- transaction preferred
- if not transactional, return partial failure and mark connector cleanup/retry path

## Failure: duplicate connect request
Behavior:
- idempotent success preferred
- return existing mapping if already connected

---

# 14. Transaction and Idempotency Guidance

The connect flow should be treated as a transactional orchestration.

Recommended order inside one transaction when possible:
1. validate board access
2. resolve Clawn agent
3. upsert `agents`
4. upsert `agent_connectors`
5. upsert `board_agents`
6. commit

Idempotency rule:
- connecting the same Clawn agent to the same board twice should not create duplicate board memberships or duplicate primary connectors

---

# 15. Suggested Internal Components

## Backend
- `ClawnAgentProvider`
  - lists agents for current user
  - resolves detailed runtime metadata

- `ConnectClawnAgentService`
  - validates ownership
  - maps source payload into MoziBoard records
  - performs upserts/transaction

- `MoziBoardAgentMapper`
  - converts Clawn agent metadata into `agents` + `agent_connectors` shape

## Frontend
- `ClawnAgentPickerModal`
- `ClawnAgentList`
- `ConnectAgentPermissionForm`
- `useClawnAgents(boardId)`
- `useConnectClawnAgent()`

---

# 16. Phase Plan

## Phase 1 — Internal ecosystem MVP
- MoziBoard can list Clawn agents for current user
- user can connect one Clawn agent to a board
- server auto-creates connector config
- board membership appears immediately
- ops dashboard can inspect connector/event state

## Phase 2 — Operational polish
- reconnect action
- disconnect/detach action
- already connected filters
- health badges from runtime state
- better empty/error states

## Phase 3 — Multi-runtime evolution
- support non-internal native runtimes
- support remote HTTP Clawn runtimes
- support pull/relay transport
- unify picker model across Clawn, manual, and future runtimes

---

# 17. Recommended MVP Scope

To ship quickly without over-designing:

## In scope
- list current user's Clawn agents
- only show runtime-ready agents
- connect one agent to one board
- internal native connector only
- machine-token auth only
- board permission form
- idempotent connect behavior

## Out of scope for first pass
- bulk connect
- cross-org shared agents
- remote HTTP runtimes
- pull relay setup
- runtime token rotation UI
- bi-directional settings sync back into Clawn

---

# 18. Example MVP Sequence

## User-visible flow
1. User opens board
2. User clicks `Register Agent`
3. User chooses `From Clawn`
4. MoziBoard lists eligible Clawn agents
5. User selects `Kodinger 5.4`
6. User enables `auto accept tasks`
7. User clicks `Connect Agent`
8. Board shows `Kodinger 5.4` as attached
9. Ops Dashboard shows connector as `connected`

## Server flow
1. `GET /api/integrations/clawn/agents?board_id=...`
2. user picks `clawn-agent-123`
3. `POST /api/integrations/clawn/connect`
4. MoziBoard resolves Clawn runtime metadata
5. MoziBoard upserts agent + connector + board membership
6. response returns success payload

---

# 19. Why This Matters Strategically

This flow is not just a convenience feature.
It is the bridge between:
- **Clawn as the runtime ecosystem**
- **MoziBoard as the mission control layer**

If this flow is smooth, then MoziBoard becomes the operational cockpit while Clawn remains the easiest native source of agent supply.

That creates a clean ecosystem story:
- create/manage your agents in Clawn
- assign/orchestrate them in MoziBoard

This is the right separation of responsibility.

---

# 20. Recommendation

Build this as a first-class **Agent Picker**, not as a manual connector helper.

The implementation priority should be:
1. backend integration facade for Clawn agent listing + connect
2. Register Agent modal with Clawn source picker
3. idempotent auto-connect orchestration
4. ops visibility after connection

That gives MoziBoard the first real native user flow for ecosystem-grade agent onboarding.
