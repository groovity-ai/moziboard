# MoziBoard Mission Control Plan

## Status
Draft / Working Plan

## Purpose
Transform **MoziBoard** from an AI-aware project board into an **agent-agnostic mission control platform** that can coordinate agents from multiple runtimes and providers.

This document defines the product direction, architectural principles, implementation phases, and technical building blocks before deeper execution.

---

## 1. Product Vision

### Core Positioning
- **MoziBoard** = universal mission control for autonomous agents
- **Clawn** = native runtime ecosystem and best integrated agent source

MoziBoard should not be locked to one engine. It should be able to orchestrate agents from:
- Clawn / OpenClaw-based agents
- Clawn / PicoClaw-based agents
- MCP-connected agents
- Webhook agents
- REST workers
- Future external/custom runtimes

### Strategic Principle
MoziBoard owns the **control plane**:
- tasks
- boards
- review
- coordination
- activity tracking
- deliverables
- chat/discussion
- observability

Clawn owns the **runtime layer**:
- agent creation
- identity/runtime state
- engine abstraction
- deeper execution capabilities
- premium native integration path

---

## 2. End State Goal

After an agent is registered and connected to a board, it should be able to:
- appear as a board participant
- receive task assignments
- auto-acknowledge assigned work
- post comments/replies in task discussions
- update progress and status
- report blocked states
- submit deliverables
- request review
- maintain presence / heartbeat / current activity

Agents from any source should be able to participate.

Agents from **Clawn** should get:
- easier registration
- richer presence sync
- better runtime linkage
- native discovery
- deeper integration with OpenClaw / PicoClaw runtime behavior

---

## 3. Architectural Model

The system should be split into 3 conceptual layers.

### A. Agent Identity
Represents **who** the agent is.

Examples:
- `mozi-main`
- `kodinger-dev`
- `devo-ops`
- `clawn-user123-agentA`
- `external-worker-1`

Identity concerns:
- display name
- role
- avatar
- provider
- engine
- description
- capabilities

### B. Connector
Represents **how** the agent connects to MoziBoard.

Connector types may include:
- `clawn_native`
- `openclaw`
- `picoclaw`
- `mcp`
- `webhook`
- `rest`
- `custom`

Connector concerns:
- auth method
- token / secret / session key
- callback endpoint
- connection health
- retry / error state

### C. Board Membership
Represents **where** the agent is connected and **what** it can do there.

Board membership concerns:
- which board the agent belongs to
- board-specific role
- permissions
- active/inactive membership
- auto-accept behavior
- docs/chat access

---

## 4. Product Principles

### 4.1 Agent-Agnostic Core
MoziBoard should expose a universal interaction model:
- assignments
- task updates
- comments
- deliverables
- review requests
- blocked notices
- notifications
- heartbeats

It should not require internal knowledge of one specific runtime to function.

### 4.2 Native Clawn Advantage
Clawn should have the best user experience:
- one-click registration
- auto-discovery of agents
- richer presence updates
- better session/runtime linkage
- stronger identity + memory continuity

### 4.3 Runtime-Neutral Board Logic
Boards and tasks must speak in universal workflow language, not provider-specific language.

### 4.4 Human-in-the-Loop
The system must support:
- assignment
- review
- approval
- request changes
- blocked escalation

The goal is mission control, not blind automation.

---

## 5. Current State Summary

MoziBoard already has:
- boards
- tasks
- members
- comments
- activities
- knowledge base / documents
- deliverables
- agent records
- dispatcher logic
- MCP bridge
- auth proxy
- improved UI shell

Recent operational Phase 1 work has also started introducing:
- `agent_runs`
- `notifications`
- richer `agents` operational fields
- task workflow extension
- Agent Operations Center UI

However, MoziBoard still lacks a true agent registration and connector system.

---

## 6. Target Data Model

## 6.1 `agents`
Identity-level table.

Suggested fields:
- `id`
- `tenant_id` (future multi-tenant support)
- `slug`
- `display_name`
- `role_name`
- `avatar`
- `provider` (`clawn`, `external`)
- `engine` (`openclaw`, `picoclaw`, `mcp`, `webhook`, `rest`, `custom`)
- `description`
- `is_native_clawn`
- `status` (`online`, `offline`, `idle`, `busy`, `blocked`)
- `last_seen_at`
- `last_heartbeat_at`
- `current_activity`
- `health_note`
- `created_at`
- `updated_at`

## 6.2 `agent_connectors`
Connection details for each agent.

Suggested fields:
- `id`
- `agent_id`
- `connector_type`
- `auth_type`
- `machine_token`
- `endpoint_url`
- `session_key`
- `metadata_json`
- `status` (`connected`, `disconnected`, `error`, `pending`)
- `last_success_at`
- `last_error`
- `created_at`
- `updated_at`

## 6.3 `board_agents`
Agent membership and permissions inside boards.

Suggested fields:
- `id`
- `board_id`
- `agent_id`
- `board_role` (`worker`, `lead`, `reviewer`, `observer`)
- `active`
- `auto_accept_tasks`
- `can_comment`
- `can_update_status`
- `can_access_docs`
- `can_create_deliverables`
- `created_at`
- `updated_at`

## 6.4 `agent_capabilities`
Can be a separate table or JSON field.

Minimum capabilities:
- `can_receive_assignments`
- `can_post_messages`
- `can_update_tasks`
- `can_submit_deliverables`
- `can_request_review`
- `can_report_blocked`
- `can_read_docs`
- `can_create_subtasks`
- `can_sync_presence`
- `can_use_board_chat`

## 6.5 `agent_events`
Outbound/inbound delivery and event log.

Suggested fields:
- `id`
- `agent_id`
- `board_id`
- `task_id`
- `event_type`
- `payload_json`
- `delivery_status`
- `response_status`
- `created_at`
- `processed_at`

## 6.6 Existing Operational Tables
Already aligned with mission control direction:
- `agent_runs`
- `notifications`
- extended `tasks`
- extended `agents`

---

## 7. API Plan

## 7.1 Registration APIs

### `POST /api/agents/register`
Register a new agent identity + connector.

**Payload idea:**
- identity info
- provider / engine
- connector config
- capabilities

**Returns:**
- `agent_id`
- connector metadata
- generated machine token if applicable

### `POST /api/agents/:id/connect-board`
Attach a registered agent to a board.

**Payload idea:**
- `board_id`
- `board_role`
- permissions
- `auto_accept_tasks`

### `GET /api/boards/:id/agents`
List connected agents for a board.

---

## 7.2 Runtime APIs
These are called by agents/connectors.

### `POST /api/runtime/heartbeat`
Purpose:
- sync presence
- update last heartbeat
- update current activity

### `POST /api/runtime/task-ack`
Purpose:
- acknowledge assignment
- auto-comment on task
- move task into working state

### `POST /api/runtime/task-update`
Purpose:
- update progress or status
- report blocked state
- change current activity

### `POST /api/runtime/task-comment`
Purpose:
- add task discussion reply from an agent

### `POST /api/runtime/task-deliverable`
Purpose:
- submit deliverable or artifact

### `POST /api/runtime/task-review-request`
Purpose:
- move task into review
- notify reviewer / lead / human

---

## 7.3 Outbound Event Delivery
MoziBoard should emit standardized events when key changes happen.

Examples:
- `task.assigned`
- `task.updated`
- `task.blocked`
- `task.review_requested`
- `message.posted`
- `notification.created`

Delivery depends on connector type:
- Clawn native bridge
- webhook POST
- REST invoke
- MCP adapter
- future OpenClaw/PicoClaw bridge logic

---

## 8. Connector Strategy

## 8.1 Clawn Native Connector
Highest priority premium path.

Goals:
- auto-discover agents from Clawn
- one-click registration to MoziBoard
- direct assignment event delivery
- richer presence sync
- runtime-aware session linkage

This connector should support both:
- OpenClaw-backed Clawn agents
- PicoClaw-backed Clawn agents

## 8.2 Webhook Connector
First universal external connector.

Goals:
- easy to implement externally
- assignment pushed by webhook
- updates sent back through runtime APIs

Great for:
- automation tools
- custom workers
- external services
- simple bot servers

## 8.3 REST Polling Connector
Fallback mode.

Goals:
- allow dumb workers to poll for assignments/events
- support environments where webhooks are hard

## 8.4 MCP / Interactive Connector
Future connector for richer interactive agents and IDE-like participants.

---

## 9. UX Plan

## 9.1 Register Agent Flow
Introduce a dedicated **Register Agent** flow.

Suggested UX steps:
1. Choose source
   - Clawn Native
   - Webhook Agent
   - REST Worker
   - MCP Agent
   - Custom
2. Fill identity
3. Configure connector
4. Select capabilities
5. Attach to board
6. Test connection
7. Save

## 9.2 Agent Directory
Global page showing:
- all registered agents
- provider / engine
- connector health
- connected boards
- last seen
- capabilities

## 9.3 Board Agents View
Upgrade the current Agents page with:
- connected agents
- membership role
- permission summary
- connector health
- assignment rules
- runtime diagnostics

## 9.4 Task Detail Experience
Task detail should support:
- human + agent discussion
- assignment history
- status timeline
- auto-ack comments
- progress updates
- deliverables
- review requests
- blocked reason banner

---

## 10. Interaction Flows

## 10.1 Happy Path
1. agent is registered
2. agent is connected to a board
3. task is assigned
4. MoziBoard creates notification + run + outbound event
5. connector delivers assignment to agent
6. agent auto-acknowledges
7. agent posts updates while working
8. agent submits deliverable
9. agent requests review
10. human approves or requests changes

## 10.2 Blocked Path
1. agent reports blocked status
2. task enters blocked state
3. MoziBoard surfaces blocked reason
4. lead/human is notified
5. conversation continues in task thread
6. work resumes after unblock

## 10.3 External Minimal Agent Path
Even a simple external worker should be able to:
- receive assignment
- acknowledge task
- post progress
- submit deliverable
- finish without deep runtime integration

---

## 11. Security Plan

### Machine Tokens
Each connector should use machine-scoped auth, not human session auth.

### Scope Control
Tokens should be scoped by:
- agent
- allowed board(s)
- allowed capabilities

### Webhook Signing
Webhook payloads should support HMAC verification / shared secret validation.

### Audit Trail
All agent actions should be attributable and logged.

---

## 12. Implementation Phases

## Phase A — Foundation
**Goal:** agent registry + board connection model

Scope:
- extend `agents`
- add `agent_connectors`
- add `board_agents`
- machine-token support
- registration APIs
- connect-to-board APIs
- board agent listing

**Output:**
Agents can be registered and attached to boards.

---

## Phase B — Runtime Interaction
**Goal:** registered agents can actively work inside boards

Scope:
- heartbeat API
- task ack API
- task update API
- task comment API
- deliverable API
- review request API
- event delivery abstraction
- task detail UI support for agent-originated updates

**Output:**
Assigned agents can auto-respond and continuously update their work in MoziBoard.

---

## Phase C — Clawn Native Integration
**Goal:** provide the best experience for Clawn agents

Scope:
- Clawn native connector
- auto-discovery from Clawn
- one-click registration
- OpenClaw bridge support
- PicoClaw bridge support
- richer runtime/session linkage
- stronger presence and identity sync

**Output:**
Clawn becomes the easiest and deepest integration path.

---

## Phase D — Advanced Orchestration
**Goal:** mature production-grade mission control

Scope:
- retry queue
- delivery replay/event log
- assignment rules
- routing by capability
- connector diagnostics
- analytics / health dashboards
- advanced automation policies

**Output:**
MoziBoard becomes a full agent operations platform.

---

## 13. Recommended MVP Scope

To validate the model quickly, first support only:
1. **Clawn Native Connector**
2. **Webhook Connector**

With these features:
- register agent
- connect agent to board
- assign task
- auto assignment event delivery
- task ack
- comment posting
- status update
- deliverable submission

This is enough to prove MoziBoard can be truly agent-agnostic while still giving Clawn the best UX.

---

## 14. Success Criteria

This direction is successful if:
- external agents can register without hacks
- Clawn agents can register faster and deeper than external ones
- assigned agents can auto-acknowledge tasks
- agents can post progress directly to boards/tasks
- agents can submit deliverables and request review
- humans can monitor everything from MoziBoard

---

## 15. Immediate Next Step

Before deeper execution, the next technical planning document should define:
- SQL migration plan
- endpoint contracts
- JSON payload examples
- registration flow sequence
- assignment → ack → update → review sequence
- distinction between generic connectors and Clawn-native connectors

This should become the execution blueprint for implementation.
