# Clawn Native v2 Plan

## Status
Working implementation note

## Purpose
This note defines the next connector refactor needed to make `clawn_native` topology-aware without locking MoziBoard to a single deployment shape.

It is intentionally short and execution-focused.

---

## Problem
Current `clawn_native` support is still too ambiguous.

Right now, `clawn_native` can mean several very different things:
- an OpenClaw runtime on the same server
- an OpenClaw runtime on another server
- a local runtime on a laptop / device behind NAT

If these cases are not separated now, the connector model will drift and later runtime dispatch logic will become fragile.

---

## Decision
Keep `connector_type = clawn_native` as the product-level connector family, but add transport semantics underneath it.

### New connector concepts
- `transport_mode`
- `base_url`
- `agent_ref`
- keep `session_key`
- keep flexible extension data inside `metadata_json`

This allows one connector family to support multiple topologies without changing the product surface each time.

---

## Transport Modes

### 1. `internal`
Use when MoziBoard can resolve the runtime directly within the same host / private network / trusted local environment.

Expected shape:
- optional `session_key`
- optional `agent_ref`
- no public URL required

Use case:
- same server deployment
- private runtime bridge

---

### 2. `remote_http`
Use when the runtime lives on another server and can be reached over HTTP(S).

Expected shape:
- `base_url` required
- optional `agent_ref`
- optional `session_key`
- machine-to-machine auth required

Use case:
- OpenClaw installed on another VPS
- separate self-hosted runtime
- dedicated runtime node

---

### 3. `pull`
Use when the runtime is local to a user device or not directly reachable from MoziBoard.

Expected shape:
- optional `agent_ref`
- optional `device_id` or node identifier in `metadata_json`
- runtime polls or opens outbound relay later

Use case:
- laptop-local runtime
- home machine behind NAT
- temporary dev environment

`pull` is defined now for schema consistency, but live relay/orchestration does not need to be built in this step.

---

## Schema Changes

### Table: `agent_connectors`
Add fields:
- `transport_mode` TEXT DEFAULT `'internal'`
- `base_url` TEXT DEFAULT `''`
- `agent_ref` TEXT DEFAULT `''`

Keep existing fields:
- `connector_type`
- `auth_type`
- `machine_token_hash`
- `endpoint_url`
- `session_key`
- `metadata_json`
- `status`

### Notes
- `endpoint_url` remains useful for `webhook`
- `base_url` is for runtime-level remote access, not generic callback delivery
- `agent_ref` should identify the runtime-side agent/session/worker reference without forcing one naming scheme

---

## API Changes

### `POST /api/agents/register`
Expand `connector` payload to support:
- `transport_mode`
- `base_url`
- `agent_ref`
- existing `session_key`

### Example: Clawn native internal
```json
{
  "provider": "clawn",
  "engine": "openclaw",
  "is_native_clawn": true,
  "connector": {
    "connector_type": "clawn_native",
    "transport_mode": "internal",
    "auth_type": "machine_token",
    "session_key": "agent:main:main",
    "agent_ref": "mozi-prod"
  }
}
```

### Example: Clawn native remote HTTP
```json
{
  "provider": "clawn",
  "engine": "openclaw",
  "is_native_clawn": true,
  "connector": {
    "connector_type": "clawn_native",
    "transport_mode": "remote_http",
    "auth_type": "machine_token",
    "base_url": "https://runtime.example.com",
    "agent_ref": "mozi-remote",
    "session_key": "agent:main:main"
  }
}
```

### Example: Clawn native pull
```json
{
  "provider": "clawn",
  "engine": "openclaw",
  "is_native_clawn": true,
  "connector": {
    "connector_type": "clawn_native",
    "transport_mode": "pull",
    "auth_type": "machine_token",
    "agent_ref": "mozi-local",
    "metadata": {
      "device_id": "macbook-mirza"
    }
  }
}
```

---

## UI Changes

Update Register Agent UI so Clawn Native registration can choose:
- `This server` → `internal`
- `Remote OpenClaw URL` → `remote_http`
- `Local / Pull mode` → `pull`

Expected form behavior:
- `internal` shows `session_key` and optional `agent_ref`
- `remote_http` shows `base_url`, optional `session_key`, optional `agent_ref`
- `pull` shows optional `agent_ref` and a short explanation that runtime-driven polling/relay comes later

---

## Scope Now
This step should include:
1. doc + naming lock
2. DB schema extension
3. backend register API update
4. UI form update
5. persistence + listing verification

---

## Not In Scope Yet
Do not implement yet:
- websocket relay
- background bridge daemon
- full remote dispatch to another runtime
- token rotation UX
- health syncing across remote runtimes

Those should come after the topology-aware connector model is stable.

---

## Expected Outcome
After this step:
- `clawn_native` is no longer ambiguous
- MoziBoard can represent internal, remote, and pull-ready native runtimes cleanly
- future bridge work can build on a stable connector contract instead of rewriting the registration model later
