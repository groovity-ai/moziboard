# Clawn Agent Picker → MoziBoard Connect Flow

Last updated: 2026-03-10

This document defines the **schema-verified** product and technical blueprint for a one-click **"Connect from Clawn"** flow inside MoziBoard.

This revision is based on an audit of the current Clawn backend and database.

## Audit-verified reality
Clawn currently appears to be:
- **project-centric**, not agent-registry-centric
- ownership-scoped through `projects.user_id`
- runtime-aware at the **project/container** level
- capable of exposing project status, engine, capabilities, config, and basic agent status

Clawn does **not yet** expose a formal stable agent registry contract with guaranteed fields such as:
- `clawn_agent_id`
- `agent_ref`
- `session_key`
- durable runtime linkage records suitable for third-party orchestration

Because of that, this blueprint treats the source object as:

> **Clawn Project / Runtime**

not as a fully normalized multi-agent registry.

The UX may still say **"Connect Agent from Clawn"** for simplicity, but the backend contract for Phase 1 should be based on **projects**.

---

# 1. Problem Statement

Today, connecting a Clawn-backed runtime to MoziBoard requires internal knowledge such as:
- connector type
- transport mode
- machine token lifecycle
- runtime auth expectations
- whether runtime session linkage is stable or only convention-based

That is acceptable for development, but not for real product UX.

From the user's point of view, the intent is simple:

> "Show me my Clawn runtime projects and let me connect one to this board."

The user should not be required to understand transport internals.

---

# 2. Product Goal

Turn manual runtime registration into a **source picker flow**.

Inside MoziBoard, the user should be able to:
- open **Register Agent**
- choose **Clawn** as the source
- see only Clawn projects/runtimes they are allowed to use
- select one item
- choose board permissions
- click **Connect**

MoziBoard should then automatically:
- resolve the selected Clawn project's runtime metadata
- create or update the MoziBoard `agents` record
- create or update the `agent_connectors` record
- attach the agent to the board through `board_agents`
- generate/store runtime auth material where required

---

# 3. Why the Source Object Must Be "Project" for Phase 1

## 3.1 What the Clawn audit confirmed
The following are real, verified Clawn concepts today:
- `projects`
- `users`
- `projects.user_id` ownership
- `projects.engine`
- `projects.status`
- `projects.container_id`
- `projects.container_name`
- project-level `config`
- project-level capabilities
- `GET /api/projects`
- `GET /api/projects/:id`
- `GET /api/projects/:id/agent-status`
- `GET /api/projects/:id/sessions`
- `GET /api/projects/:id/config`

## 3.2 What the audit did not confirm as a stable integration contract
The following should **not** be assumed as first-class Clawn backend contracts yet:
- separate `agents` table for user-owned runtime agents
- stable `clawn_agent_id` record independent of project
- persisted `agent_ref` field exposed as integration-safe metadata
- persisted `session_key` field exposed as integration-safe metadata
- runtime connector records suitable for direct MoziBoard ingestion

## 3.3 Implication
Phase 1 must treat:
- **one Clawn project**

as:
- **one connectable runtime-backed agent source**

This is slightly less ideal than a true agent registry, but it is much safer and aligns with the real system today.

---

# 4. UX Vision

## 4.1 Entry Point
Primary entry point:
- `Board > Agents > Register Agent`

Within the Register Agent flow, the user sees source options:
- Clawn
- Manual / Custom Connector
- Future sources (OpenClaw direct, webhook, remote runtime, etc.)

For this blueprint, the focus is:
- **Clawn → Project/Runtime Picker → Connect**

The UI label may still say:
- `Connect Agent from Clawn`

because that is the user-friendly mental model.

---

## 4.2 User Journey

### Step 1 — Open picker
User clicks:
- `Connect Agent`
- then chooses `From Clawn`

### Step 2 — Browse eligible Clawn runtimes
MoziBoard shows a list of **Clawn projects** available to the current user.

Each item should show:
- project name
- engine (`openclaw`, `picoclaw`, etc.)
- status
- plan
- capabilities
- basic readiness status
- already connected badge if this board already has it

Optional enriched fields if available:
- display name for main runtime agent
- model
- last seen / last heartbeat
- health summary

### Step 3 — Select one runtime source
User selects one item.

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
MoziBoard performs backend orchestration:
- validate ownership/access
- fetch project detail from Clawn
- resolve runtime metadata as far as Clawn currently supports
- upsert internal agent record
- upsert connector config
- create board membership
- return success state

### Step 7 — Board reflects connected runtime-backed agent
User is returned to the board agent list and can immediately:
- assign work
- see the attached agent in board membership
- inspect connector health in Ops Dashboard

---

# 5. Core Principle: User Picks Intent, System Builds Runtime Config

The user should choose:
- **which Clawn runtime/project**
- **which board**
- **what permissions**

The system should decide:
- connector type
- transport mode
- auth mode
- internal mapping to MoziBoard agent identity
- token generation/storage
- whether enough runtime linkage exists to enable dispatch immediately

This is the core product rule for the Clawn native path.

---

# 6. Data Sources and Integration Boundary

There are two ways for MoziBoard to discover Clawn runtime sources.

## Option A — Direct internal adapter (recommended for Phase 1)
MoziBoard reads Clawn-owned data through a trusted internal adapter.

This adapter may use:
- direct database reads
- internal authenticated API calls
- service-to-service fetches

Use this when both systems are controlled by the same ecosystem and shipping speed matters.

### Pros
- fastest to ship
- enough for internal rollout
- works with current project-centric Clawn model

### Cons
- tighter coupling to current Clawn schema
- schema drift risk if Clawn changes aggressively

---

## Option B — Formal Clawn Integration API (recommended for later hardening)
MoziBoard fetches data from an explicit Clawn integration API.

Example responsibilities on Clawn side:
- list connectable projects for current user
- expose runtime-readiness state
- expose MoziBoard-safe runtime metadata
- later expose richer agent registry data when Clawn evolves

### Pros
- cleaner boundary
- easier long-term maintenance
- stronger security contract

### Cons
- requires more work upfront

---

# 7. Ownership and Access Model

This flow is only safe if MoziBoard can determine which Clawn projects belong to the current user.

## Required rule
MoziBoard must only list Clawn projects/runtimes that the authenticated MoziBoard user is allowed to connect.

That means we need a trust mapping between:
- MoziBoard user identity
- Clawn user identity

## Minimum acceptable access rule
A runtime source can appear in the picker only if:
- the current MoziBoard user owns the Clawn project, or
- the current user has explicit permission in Clawn, or
- the current user is an admin with cross-project visibility

## Audit-backed ownership note
The current Clawn DB supports ownership through:
- `projects.user_id`

This is the most reliable Phase 1 basis for filtering picker results.

## Never do this
Do not let the frontend pass runtime internals and assume they are trustworthy.

The selected `clawn_project_id` should be treated as a lookup key only.
MoziBoard must resolve all sensitive/runtime fields server-side.

---

# 8. Required Clawn Metadata

For each connectable Clawn runtime source, MoziBoard needs enough data to:
- render the picker
- decide whether connection is possible
- create a valid MoziBoard agent + connector mapping

## 8.1 Picker list payload
This is what the UI should use.

```json
{
  "project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
  "display_name": "aa",
  "owner_user_id": "13cd5bad-cabf-4c81-9172-e24f32edf7c7",
  "engine": "openclaw",
  "plan": "starter",
  "status": "exited",
  "container_id": "9b6045d6dfa272cb8892e8547abfbaa5e91157af0ec0dcdf8bb8e4902ce69fd5",
  "container_name": "aiagenz-ec24c0f8-d99d-4c9e-b361-0e94d4698322",
  "capabilities": ["sessions", "memory", "skills"],
  "is_connectable": true,
  "connect_reason": "project_runtime_available",
  "already_connected": false
}
```

## 8.2 Runtime resolution payload
This is what the backend should resolve before connector creation.

```json
{
  "project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
  "display_name": "aa",
  "provider": "clawn",
  "engine": "openclaw",
  "runtime": {
    "connector_type": "clawn_native",
    "transport_mode": "internal",
    "project_token": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
    "main_agent_id": "main",
    "status": "stopped",
    "session_strategy": "convention_based_or_unresolved"
  },
  "capabilities": {
    "sessions": true,
    "memory": true,
    "skills": true
  }
}
```

## Important note
This runtime payload intentionally avoids pretending that `agent_ref` and `session_key` are already stable Clawn contracts.

Those fields may exist later through:
- a dedicated Clawn integration endpoint
- a MoziBoard-safe runtime metadata contract
- a formal runtime registry layer

Until then, they should be treated as:
- unresolved
- convention-based
- or optional integration enrichments

---

# 9. Proposed MoziBoard Backend API

To keep the frontend simple, MoziBoard should expose a Clawn integration facade.

## 9.1 List eligible Clawn projects/runtimes
### `GET /api/integrations/clawn/projects?board_id=:id`

Returns connectable Clawn projects available to the current user and enriched with connection state.

### Response example
```json
[
  {
    "project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
    "display_name": "aa",
    "engine": "openclaw",
    "plan": "starter",
    "status": "exited",
    "capabilities": ["sessions", "memory", "skills"],
    "is_connectable": true,
    "connect_reason": "project_runtime_available",
    "already_connected": false,
    "existing_agent_id": null
  }
]
```

### Server responsibilities
- authenticate MoziBoard user
- resolve corresponding Clawn identity
- fetch Clawn projects owned by that user
- filter to projects that are connectable for the current integration phase
- annotate if already connected to current board

---

## 9.2 Connect selected Clawn runtime/project
### `POST /api/integrations/clawn/connect`

### Request body
```json
{
  "board_id": "5dd8c641-f7af-46b1-91bd-ada0d6384fa0",
  "clawn_project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
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
    "id": "clawn-project:ec24c0f8-d99d-4c9e-b361-0e94d4698322",
    "display_name": "aa"
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
    "agent_id": "clawn-project:ec24c0f8-d99d-4c9e-b361-0e94d4698322"
  }
}
```

---

# 10. Proposed Server-Side Connect Algorithm

When `POST /api/integrations/clawn/connect` is called, MoziBoard should do the following:

## Step 1 — Authenticate user
Resolve currently logged-in MoziBoard user.

## Step 2 — Resolve Clawn identity
Map the MoziBoard user to the Clawn user/account context.

## Step 3 — Validate board access
Ensure the user is allowed to modify the target board.

## Step 4 — Fetch selected project from Clawn
Resolve the selected `clawn_project_id` from trusted Clawn data.

## Step 5 — Validate ownership and readiness
Ensure:
- project belongs to the current user or accessible scope
- project/runtime is connectable
- required integration fields are present
- engine/runtime type is supported

## Step 6 — Upsert MoziBoard `agents`
Create or update a normalized MoziBoard agent identity representing this Clawn project runtime.

Recommended ID strategy:
- `clawn-project:<clawn_project_id>`

This avoids collisions and reflects current Clawn reality.

Fields to sync:
- `id`
- `display_name`
- `provider = clawn`
- `engine = openclaw | picoclaw | ...`
- minimal presence/status metadata when available

## Step 7 — Upsert `agent_connectors`
Create or update one native connector using server-resolved runtime metadata.

Recommended defaults for initial Clawn integration:
- `connector_type = clawn_native`
- `auth_type = machine_token`
- `status = connected`

### Transport note
For Phase 1, transport mode should only be marked `internal` if MoziBoard can resolve a stable dispatch path.

If Clawn still does not expose stable runtime linkage, then the integration should either:
- store the connector as partially configured but not dispatchable yet, or
- use a reduced MVP where connection means "registered for future runtime bridge", not immediate event delivery

This is safer than faking runtime linkage.

## Step 8 — Store source metadata
Example metadata JSON:
```json
{
  "source": "clawn",
  "clawn_project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
  "engine": "openclaw",
  "status": "exited",
  "linkage_mode": "project-centric"
}
```

## Step 9 — Upsert `board_agents`
Attach the runtime-backed agent to the selected board with the chosen permissions.

## Step 10 — Return hydrated result
Return the normalized agent + connector + board membership summary for the UI.

---

# 11. Runtime Linkage Reality and Gap

## 11.1 What is known today
Audit findings indicate that Clawn exposes:
- project-level identity
- engine
- status
- project config
- basic `agent-status` with agent id `main`
- session listing endpoint

## 11.2 What is not safe to assume yet
Do not assume that MoziBoard can safely and durably derive:
- `agent_ref`
- `session_key`

from current Clawn backend as a stable integration contract.

There are hints of conventions in the frontend and current runtime design, but conventions are not enough for a robust connector contract.

## 11.3 Recommended bridge contract to add in Clawn
To unlock clean native dispatch, Clawn should eventually expose a dedicated endpoint such as:

### `GET /api/projects/:id/moziboard-runtime`

Example response:
```json
{
  "project_id": "ec24c0f8-d99d-4c9e-b361-0e94d4698322",
  "display_name": "aa",
  "engine": "openclaw",
  "status": "running",
  "connectable": true,
  "connector_type": "clawn_native",
  "transport_mode": "internal",
  "main_agent_id": "main",
  "agent_ref": "main",
  "session_key": "agent:main:main",
  "linkage_confidence": "explicit"
}
```

Once Clawn exposes something like this, MoziBoard can safely upgrade from project-centric integration to runtime-native dispatch.

---

# 12. Database Mapping Strategy

## 12.1 Agent identity
MoziBoard should treat Clawn as a source of runtime-backed agent identity.

Recommended internal mapping for current reality:
- `agents.id = clawn-project:<clawn_project_id>`
- `agents.provider = clawn`
- `agents.engine = <engine from Clawn project>`

This is explicit and future-safe.

---

## 12.2 Connector mapping
One Clawn project should typically map to one primary native connector.

Recommended connector uniqueness rule for MVP:
- one active primary connector per `(agent_id, connector_type, transport_mode)`

If reconnecting the same Clawn project:
- update existing connector instead of creating duplicates when possible

---

## 12.3 Board membership mapping
`board_agents` remains board-scoped.

This means one Clawn-backed runtime source can be attached to multiple boards later, subject to product rules.

For MVP, it is acceptable to allow the same source on multiple boards if the user intentionally connects it.

---

# 13. UI Blueprint

## 13.1 Modal structure
### Modal title
`Connect Agent from Clawn`

### Section A — Source list
Search + filterable list of Clawn projects/runtimes.

Each card should show:
- display name / project name
- engine
- plan
- status badge
- capabilities summary
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

## 13.2 Empty states
### No Clawn runtimes found
Show:
- `No Clawn runtime projects found`
- `Create or start a project in Clawn first, then come back here.`

### Project exists but not connectable
Show disabled item with reason, such as:
- project not running
- runtime bridge metadata missing
- unsupported engine for current MVP
- ownership mismatch

---

## 13.3 Success state
After connect success:
- close modal
- refresh board agent list
- optionally show success toast
- optionally deep-link to Ops Dashboard for inspection

---

# 14. Security Rules

## 14.1 Frontend never submits runtime internals
The frontend should never post:
- `agent_ref`
- `session_key`
- `base_url`
- `machine_token`
- `shared_secret`

Those must be resolved server-side.

## 14.2 Server must verify ownership
`clawn_project_id` is not trusted by itself.
Server must re-check ownership against authenticated user context.

## 14.3 Machine token handling
If machine tokens are used for runtime callbacks:
- generate server-side only
- store only token hash in DB
- do not expose token back to the UI unless explicitly necessary
- if Clawn runtime must receive a token, pass it through a trusted server-to-server path

## 14.4 Auditability
MoziBoard should log or record:
- who connected the runtime
- which board it was connected to
- when the connector was created/updated
- what source integration was used

---

# 15. Failure Modes and UX Handling

## Failure: user no longer owns project
Return:
- `403 forbidden`
- UI message: `This Clawn project is no longer available to your account.`

## Failure: runtime metadata incomplete
Return:
- `409 conflict`
- UI message: `This project exists but is not runtime-ready for MoziBoard connection yet.`

## Failure: connector upsert succeeds but board attach fails
Behavior:
- transaction preferred
- if not transactional, return partial failure and mark connector cleanup/retry path

## Failure: duplicate connect request
Behavior:
- idempotent success preferred
- return existing mapping if already connected

---

# 16. Transaction and Idempotency Guidance

The connect flow should be treated as a transactional orchestration.

Recommended order inside one transaction when possible:
1. validate board access
2. resolve Clawn project
3. upsert `agents`
4. upsert `agent_connectors`
5. upsert `board_agents`
6. commit

Idempotency rule:
- connecting the same Clawn project to the same board twice should not create duplicate board memberships or duplicate primary connectors

---

# 17. Suggested Internal Components

## Backend
- `ClawnProjectProvider`
  - lists projects for current user
  - resolves detailed runtime metadata

- `ConnectClawnProjectService`
  - validates ownership
  - maps source payload into MoziBoard records
  - performs upserts/transaction

- `MoziBoardAgentMapper`
  - converts Clawn project/runtime metadata into `agents` + `agent_connectors` shape

## Frontend
- `ClawnProjectPickerModal`
- `ClawnProjectList`
- `ConnectAgentPermissionForm`
- `useClawnProjects(boardId)`
- `useConnectClawnProject()`

---

# 18. Phase Plan

## Phase 1 — Internal ecosystem MVP
- MoziBoard can list Clawn projects for current user
- user can connect one Clawn project/runtime to a board
- server auto-creates MoziBoard agent + connector config
- board membership appears immediately
- ops dashboard can inspect connection state
- dispatch may remain limited until Clawn exposes stronger runtime linkage

## Phase 2 — Runtime contract hardening
- Clawn exposes a MoziBoard-safe runtime metadata endpoint
- MoziBoard can resolve stable dispatch references
- true native event delivery becomes safe and explicit

## Phase 3 — Operational polish
- reconnect action
- disconnect/detach action
- already connected filters
- health badges from runtime state
- better empty/error states

## Phase 4 — Multi-runtime evolution
- support non-internal native runtimes
- support remote HTTP Clawn runtimes
- support pull/relay transport
- evolve from project-centric mapping to richer runtime/agent registry when Clawn is ready

---

# 19. Recommended MVP Scope

To ship quickly without pretending Clawn is more mature than it is:

## In scope
- list current user's Clawn projects
- show engine/status/capabilities
- connect one project/runtime source to one board
- project-centric MoziBoard agent identity
- board permission form
- idempotent connect behavior

## Out of scope for first pass
- true multi-agent-per-project registry
- bulk connect
- cross-org shared runtimes
- remote HTTP runtime support
- pull relay setup
- token rotation UI
- deep bi-directional settings sync into Clawn

---

# 20. Example MVP Sequence

## User-visible flow
1. User opens board
2. User clicks `Register Agent`
3. User chooses `From Clawn`
4. MoziBoard lists eligible Clawn projects
5. User selects `aa`
6. User enables `auto accept tasks`
7. User clicks `Connect Agent`
8. Board shows the connected Clawn-backed agent source
9. Ops Dashboard shows connector registration state

## Server flow
1. `GET /api/integrations/clawn/projects?board_id=...`
2. user picks `clawn_project_id`
3. `POST /api/integrations/clawn/connect`
4. MoziBoard resolves Clawn project/runtime metadata
5. MoziBoard upserts agent + connector + board membership
6. response returns success payload

---

# 21. Strategic Significance

This flow is the bridge between:
- **Clawn as the runtime ecosystem**
- **MoziBoard as the mission control layer**

Even with the current project-centric limitation, this is still strategically correct.

It creates a practical ecosystem story:
- create/manage your runtime projects in Clawn
- attach and orchestrate them in MoziBoard

Later, when Clawn grows a stronger runtime or agent registry, MoziBoard can upgrade the integration without throwing away the UX.

---

# 22. Recommendation

Build this as a first-class **Clawn source picker**, but implement it against **project-backed runtime sources** for now.

Implementation priority should be:
1. backend integration facade for Clawn project listing + connect
2. Register Agent modal with Clawn source picker
3. idempotent auto-connect orchestration
4. Clawn runtime metadata contract hardening
5. native dispatch once runtime linkage is explicit and safe

That gives MoziBoard a real native onboarding flow without lying to itself about Clawn's current backend maturity.
