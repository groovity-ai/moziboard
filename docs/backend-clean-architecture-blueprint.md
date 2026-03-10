# MoziBoard Backend Clean Architecture Blueprint

Last updated: 2026-03-10

## Why now

MoziBoard backend has crossed the point where a single-file `main.go` remains productive. The current backend mixes:
- bootstrap and infra init
- schema setup / migrations-ish logic
- HTTP route wiring
- business rules
- SQL persistence
- realtime broadcasting
- background worker delivery
- Clawn integration facades
- runtime auth and agent orchestration

This makes feature work fast in the short term, but increasingly expensive to maintain.

The goal is **not** to do a ceremonial enterprise rewrite.
The goal is to move MoziBoard into a **pragmatic clean architecture** that preserves shipping speed while restoring clarity.

---

## Architecture goals

### Product goals
- keep feature velocity high
- make mission-control features safer to extend
- isolate agent/runtime/ops logic so future work does not keep colliding
- make backend easier for sub-agents and contributors to navigate

### Engineering goals
- reduce `main.go` into a thin composition root
- separate domains cleanly
- keep SQL explicit and close to domain modules
- isolate side effects and background delivery pipelines
- prepare the codebase for tests and stronger auth boundaries

---

## Guiding principles

1. **Domain-first structure, not utility-folder chaos**
   - prefer `task`, `agent`, `runtime`, `ops`, `board`, `doc`, `integration/clawn`
   - avoid one giant global `handlers/ services/ repositories/` dump

2. **Pragmatic clean architecture**
   - no forced abstraction everywhere
   - only introduce interfaces where a real seam exists
   - keep pgx + SQL explicit

3. **Composition root in one place**
   - app setup happens in `cmd/server/main.go`
   - infrastructure is initialized there
   - routes are registered there

4. **Business rules live in services, not handlers**
   - handlers parse HTTP and shape responses
   - services coordinate business flow
   - repositories own persistence details

5. **Shared infra isolated from product logic**
   - DB, AI, Redis, websocket hub, config, and HTTP setup become reusable platform modules

6. **Strangler refactor, not big-bang rewrite**
   - move module by module while app remains working
   - old code can coexist temporarily until each module is extracted

---

## Target backend structure

```txt
backend/
  cmd/
    server/
      main.go

  internal/
    platform/
      config/
        config.go
      db/
        db.go
      ai/
        ai.go
      cache/
        redis.go
      httpapp/
        app.go

    realtime/
      hub.go

    modules/
      boards/
        model.go
        repository.go
        service.go
        handler.go
      docs/
        model.go
        repository.go
        service.go
        handler.go
      tasks/
        model.go
        repository.go
        service.go
        handler.go
      agents/
        model.go
        repository.go
        service.go
        handler.go
      runtime/
        service.go
        handler.go
      ops/
        service.go
        handler.go
      integrations/
        clawn/
          model.go
          service.go
          handler.go

    worker/
      dispatcher.go
      event_delivery.go
```

---

## Domain boundaries

### 1. Boards
Owns:
- boards
- board members
- base board listing / creation
- low-risk CRUD around board context

### 2. Docs
Owns:
- board documents
- doc CRUD
- doc search
- doc embedding update

### 3. Tasks
Owns:
- task CRUD
- status normalization
- assignment flow
- activity logging
- deliverables
- comments
- task search
- embedding update
- reorder flow (future bulk endpoint)

### 4. Agents
Owns:
- agent registry
- connectors
- board_agents
- agent profile / runs / notifications

### 5. Runtime
Owns:
- machine token auth
- heartbeat / ack / update / deliverable / review request
- runtime-facing orchestration rules

### 6. Ops
Owns:
- event queue inspection
- connector health inspection
- retry / requeue
- connector enable / disable

### 7. Integrations / Clawn
Owns:
- Clawn facade list/connect flow
- upstream auth forwarding
- normalization of external Clawn project shape

### 8. Realtime
Owns:
- websocket client registry
- broadcast methods
- future board-scoped channels/events

### 9. Worker
Owns:
- background dispatcher
- event delivery runner
- retry/backoff processing

---

## Dependency rules

Allowed direction:
- `cmd/server` -> `platform`, `modules`, `realtime`, `worker`
- `handlers` -> `services`
- `services` -> `repositories`, `platform`, `realtime`
- `repositories` -> `pgxpool`

Avoid:
- handlers calling random global helpers in other domains
- modules reaching into unrelated module internals
- business logic directly in `main.go`

---

## Phase plan

## Phase 1 — Foundation / composition root
Goal: remove bootstrap sprawl from `main.go`.

Deliverables:
- create `platform/config`
- create `platform/db`
- create `platform/ai`
- create `platform/cache`
- create `platform/httpapp`
- create `realtime/hub`
- make `main.go` use these packages as composition root entry points

Success criteria:
- bootstrap code becomes isolated
- `main.go` gets materially smaller / clearer
- infra init no longer mixed with HTTP business functions

---

## Phase 2 — Extract low-risk modules first
Goal: validate the modular pattern on safe surfaces before touching core task/agent orchestration.

Recommended extraction order:
1. boards + members
2. docs
3. comments / deliverables read paths

Success criteria:
- route registration for extracted modules happens via package `RegisterRoutes`
- extracted handlers no longer depend on legacy globals beyond injected dependencies
- old code can remain temporarily but becomes dead code scheduled for deletion later

---

## Phase 3 — Extract task domain
Goal: move the most central product logic into a dedicated module.

Includes:
- createTask
- updateTask
- activities
- searchTasks
- deliverables
- comments
- embedding hooks
- future bulk reorder endpoint

Special care:
- preserve side effects: notification creation, agent run creation, enqueue event, activity log, broadcast
- centralize task status normalization

---

## Phase 4 — Extract agents + runtime
Goal: isolate mission-control runtime model.

Includes:
- getAgents / getAgentProfile / register / connect-board / board-agents
- runtimeHeartbeat / taskAck / taskUpdate / taskComment / taskDeliverable / reviewRequest
- runtime auth resolver
- agent run and notification queries

---

## Phase 5 — Extract ops + delivery pipeline
Goal: isolate operational internals and make them maintainable.

Includes:
- ops event + connector handlers
- retry / requeue / enable / disable controls
- background event delivery
- retry backoff helpers
- dispatcher worker extraction

---

## Phase 6 — Extract Clawn integration facade
Goal: isolate external integration complexity.

Includes:
- auth forwarding helpers
- project normalization
- facade listing
- connect facade flow
- proxy auth endpoints (if still needed inside MoziBoard)

---

## Concrete first-pass package contracts

### Boards module
- `Repository`
  - `ListBoards(ctx)`
  - `CreateBoard(ctx, input)`
  - `ListMembers(ctx)`
  - `ListBoardMembers(ctx, boardID)`
  - `AddBoardMember(ctx, boardID, memberID, role)`
  - `RemoveBoardMember(ctx, boardID, memberID)`
- `Service`
  - thin orchestration and validation
- `Handler`
  - `RegisterRoutes(app fiber.Router)`

### Docs module
- `Repository`
  - list/create/update/delete/search docs
- `Service`
  - validation + embedding update coordination
- `Handler`
  - HTTP routes only

---

## Migration approach

For each extracted module:
1. copy stable structs and logic into new package
2. inject explicit dependencies (`db`, `hub`, `clients`, `AI` helpers, etc.)
3. register module routes from package
4. swap route wiring in `main.go`
5. confirm compile/build
6. only then delete legacy implementation

This keeps risk low and avoids a rewrite freeze.

---

## What we will NOT do right now

- no full CQRS
- no event sourcing
- no microservices split
- no generic repository abstraction for everything
- no mandatory interfaces for every service/repo
- no framework-heavy dependency injection container

These would slow us down more than help us.

---

## Definition of success

The refactor is successful when:
- `main.go` becomes a thin wiring file
- infra bootstrap is isolated
- at least one low-risk domain is fully extracted and route-wired cleanly
- new contributors can find board logic without reading 2,600+ lines first
- we have a repeatable pattern ready for tasks / agents / runtime / ops extraction

---

## Immediate execution scope

After this blueprint, execute:

### Phase 1
- extract config / db / ai / cache / http app / realtime hub
- convert main bootstrap to use them

### Phase 2
- extract boards + members module first
- wire board/member routes from the new module
- leave high-risk domains in legacy code temporarily

This gives MoziBoard a real architectural spine without pausing product momentum.
