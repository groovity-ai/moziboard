# Changelog — 2026-03-09 — Internal Bridge & Dispatcher Hardening

## Summary
This update fixes a blocking issue where the internal Clawn/OpenClaw bridge worker could stall the MoziBoard dispatcher loop. It also removes the backend's dependency on shelling out to the OpenClaw CLI, replaces that path with Gateway HTTP API delivery, and adds delivery hardening plus test-data cleanup.

## What changed

### 1. Internal bridge no longer blocks the dispatcher
- Refactored `deliverPendingAgentEvents()` so events are claimed first and processed asynchronously.
- Added bounded worker concurrency using a semaphore.
- Prevented a single slow internal bridge call from freezing the main dispatcher tick.

### 2. Replaced CLI-based internal delivery with Gateway HTTP API
- Removed the backend's runtime dependency on `openclaw agent ...` shell-outs for:
  - internal bridge event delivery
  - legacy auto-dispatch task invocation
- Switched both flows to use OpenClaw Gateway's HTTP endpoint:
  - `POST /v1/chat/completions`
  - target agent via `model: openclaw:<agentId>`
  - optional stable routing via `x-openclaw-session-key`
- This makes backend invocation more reliable inside Docker and removes PATH/CLI coupling.

### 3. Fixed legacy dispatcher SQL mismatch
- Removed use of `tasks.updated_by` in the dispatcher path.
- The old query caused SQL errors after schema drift because that column no longer exists.

### 4. Added event-delivery hardening
- Added timeout requeue protection for events stuck in `processing` for too long.
- Added panic recovery around async delivery workers so panics mark events as failed rather than leaving them stuck.
- Preserved explicit delivery state transitions (`pending`, `processing`, `sent`, `failed`, `routed`, `ignored`).

### 5. Security cleanup for Gateway auth
- Removed hardcoded Gateway token fallback from backend code.
- Moved Gateway URL/token wiring to explicit container environment variables.

### 6. Test cleanup
- Removed dummy connector records used during bridge validation.
- Removed temporary `agent_events` and `agent_runs` created during testing.
- Reset test tasks back to a clean `todo` state.

## Validation

### E2E checks completed
- Verified Gateway HTTP routing to a valid test agent works.
- Verified internal bridge message delivery reaches the expected agent session.
- Verified event delivery completes without stalling the dispatcher.
- Verified backend rebuild/restart succeeded after changes.

## Commits
- `376bf76` — `Fix internal bridge worker to use gateway API`
- `a3b9e94` — `Harden agent event delivery and clean test data`

## Operational outcome
MoziBoard's internal Clawn/OpenClaw bridge is now:
- non-blocking for the dispatcher
- independent from the OpenClaw CLI binary inside the backend container
- more resilient to stuck deliveries and worker panics
- cleaner from a configuration/security perspective
