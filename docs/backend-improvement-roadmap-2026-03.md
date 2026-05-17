# MoziBoard Backend Improvement Roadmap — March 2026

## Context Snapshot

Backend MoziBoard sudah naik level signifikan lewat Workstream 1–2:

- domain layer untuk `activityfeed`, `attention`, `boardhealth`, `boardstats`, dan `home` sudah ada
- `board_stats` summary layer sudah dibuat
- auto-recompute sudah di-hook ke jalur write utama
- homepage overview sudah punya short TTL cache + freshness metadata
- runtime/ops foundation untuk connector, board agent, runtime heartbeat, task ack/update/comment/deliverable/review juga sudah ada

Artinya, fase berikutnya bukan lagi “nambah endpoint sebanyak mungkin”, tapi **menguatkan integritas workflow, operability, observability, dan scalability**.

---

## Strategic Goal

Bikin backend MoziBoard siap jadi:

1. **mission control yang reliable**, bukan sekadar demo dashboard
2. **agent runtime backend** yang tahan terhadap state drift dan delivery issue
3. **operationally debuggable system** saat jumlah board, task, agent, event, dan connector naik
4. **secure + policy-driven backend** untuk multi-agent / multi-runtime future

---

## Current Known Strengths

### Already in place
- board/task/doc/agent/runtime domain foundation
- board health + dashboard aggregation
- summary layer (`board_stats`)
- homepage overview cache + freshness metadata
- machine-token/shared-secret auth foundation
- outbound event queue / retry-ish ops spine
- backend/frontend Docker validation flow already proven

### Current limitations
- task/review lifecycle masih belum fully state-machine-driven
- task/run/agent state sync masih bisa drift di edge case
- observability masih lebih dekat ke logs daripada real operational instrumentation
- background maintenance masih minim
- permission model belum granular per action
- perf tuning sudah mulai, tapi belum diukur systematic via query plans / latency budget

---

# Priority Roadmap

## P0 — Workflow Integrity & Operational Safety

Ini yang paling penting kalau targetnya backend yang stabil dan nggak gampang “aneh” setelah volume naik.

---

### P0.1 — Formal Task State Machine

#### Goal
Ubah task lifecycle dari implicit behavior jadi explicit backend contract.

#### Problem Today
Saat ini status task, agent run, dan agent state sudah saling terkait, tapi belum ada satu aturan transisi formal yang menjaga semuanya tetap sinkron.

#### Planned Work
- buat definisi state task canonical:
  - `todo`
  - `assigned`
  - `in_progress`
  - `review`
  - `blocked`
  - `done`
  - optional future: `cancelled`, `archived`
- bikin transition map yang valid, misalnya:
  - `todo -> assigned|in_progress|blocked`
  - `assigned -> in_progress|blocked`
  - `in_progress -> review|blocked|done`
  - `review -> in_progress|done|blocked`
  - `blocked -> in_progress|review`
- reject invalid transitions di backend
- pisahkan “status move” dari “content patch” kalau perlu
- simpan reason untuk transition sensitif:
  - blocked reason
  - review decision note
  - reopen note

#### Deliverables
- `workflow` helper/module untuk task transitions
- validation dipakai di task update + runtime update paths
- test cases untuk valid/invalid transitions

#### Definition of Done
- invalid transitions ditolak konsisten
- status task tidak bisa lompat seenaknya
- runtime update ikut aturan state machine

---

### P0.2 — Review Decision Model

#### Goal
Bikin fase `review` punya semantics yang jelas.

#### Problem Today
`review` sudah ada, tapi belum jadi model decision yang eksplisit.

#### Planned Work
- tambahkan endpoint/action semantic untuk:
  - `approve`
  - `request_changes`
  - `reopen`
- putuskan apakah butuh tabel baru seperti `task_reviews`, atau cukup activity/audit dulu
- log reviewer identity + timestamp + note
- ubah transisi review jadi explicit:
  - approve → `done`
  - request changes → `in_progress`
  - blocked in review → `blocked`

#### Deliverables
- review action contract backend
- activity/audit event untuk review decisions
- optional structured table if needed

#### Definition of Done
- review bukan cuma status, tapi state dengan action semantics
- semua review decision punya trace

---

### P0.3 — Task / Agent Run / Agent State Consistency Guard

#### Goal
Kurangi state drift antara tiga entitas utama:
- `tasks`
- `agent_runs`
- `agents`

#### Problem Today
Sudah ada sync logic, tapi edge case masih mungkin:
- task selesai tapi run masih active
- agent blocked padahal task udah pindah state
- multiple active runs yang lolos di edge condition

#### Planned Work
- definisikan canonical mapping:
  - task state → expected run state
  - run state → expected agent state
- bikin reconciliation helper internal
- enforce one-active-run-per-task-agent at code path + DB-safe guard jika perlu
- evaluasi partial unique index jika schema memungkinkan
- tambahkan repair logic ringan untuk stale/inconsistent row

#### Deliverables
- reconciliation helper
- stricter invariant checks
- repair routine / maintenance hook

#### Definition of Done
- state drift jadi detectable dan recoverable
- invariant utama terdokumentasi jelas

---

### P0.4 — Public Healthcheck & Readiness Contract

#### Goal
Pisahkan healthcheck simple dari endpoint ops yang lebih meaningful.

#### Problem Today
`/api/health` sempat ketabrak auth behavior; sekarang sudah dibypass, tapi masih belum menjadi full readiness contract.

#### Planned Work
- keep `/api/health` public and cheap
- tambah optional readiness endpoint:
  - DB ping
  - Redis ping
  - maybe version/build metadata
- define expected usage:
  - health = process alive
  - readiness = dependencies ready

#### Deliverables
- `/api/health` public stable contract
- optional `/api/readiness`
- docs update in README/docs

#### Definition of Done
- infra healthcheck nggak ambiguous lagi

---

## P1 — Observability, Auditability, and Maintenance

---

### P1.1 — Structured Audit Log for Important Mutations

#### Goal
Bikin semua perubahan penting bisa dilacak jelas.

#### Planned Work
Log/audit event untuk:
- task status change
- task assignment change
- review decision
- agent connection/disconnection
- runtime auth-sensitive actions
- connector enable/disable / retry / requeue actions

#### Recommended Shape
Either:
- enrich existing `activities`, atau
- tambah dedicated `audit_logs` table kalau mau lebih proper

Suggested fields:
- actor_type (`user`, `agent`, `system`)
- actor_id
- entity_type
- entity_id
- action
- old_value_json
- new_value_json
- metadata_json
- created_at

#### Definition of Done
- perubahan penting bisa direconstruct dari audit trail

---

### P1.2 — Background Maintenance Jobs

#### Goal
Biar sistem tetap bersih dan sehat tanpa intervensi manual terus-menerus.

#### Planned Work
Implement recurring jobs untuk:
- mark agent `offline` kalau heartbeat stale
- mark runs stale / hung kalau terlalu lama tanpa update
- recompute `board_stats` periodic full safety sweep
- re-evaluate attention signals kalau ada edge case missed
- cleanup/compact dead-letter-ish event data bila perlu
- optional purge/archival strategy untuk old noisy ops data

#### Where to Run
Options:
- internal ticker in backend
- separate worker process
- cron outside app

#### Recommended Order
1. offline stale agent
2. stale run cleanup
3. periodic board_stats safety recompute
4. event maintenance

#### Definition of Done
- sistem tetap sehat walau ada missed event / runtime crash / disconnect

---

### P1.3 — Ops Metrics & Debugging Surface

#### Goal
Biar issue production gampang ditrack, bukan cuma nebak dari log.

#### Planned Work
Tambahkan metrics / debug counters untuk:
- request latency per critical endpoint
- board_stats recompute duration
- attention/home query duration
- event delivery attempts, success, failed, dead
- active agents / stale agents / blocked agents
- active runs / stale runs
- cache hit/miss for overview

#### Possible Delivery Options
- structured logs with duration fields
- metrics endpoint (Prometheus-style later)
- internal ops summary endpoint for admin UI

#### Definition of Done
- bottleneck utama bisa diidentifikasi tanpa spelunking manual berlebihan

---

## P1 — Performance Maturity

---

### P1.4 — Query Plan Audit (`EXPLAIN ANALYZE` pass)

#### Goal
Naik dari “query feels better” ke “query measured and justified”.

#### Planned Work
Run `EXPLAIN ANALYZE` on:
- homepage overview queries
- `attention.ListByUser`
- `activityfeed.ListRecentByUser`
- runtime endpoints with multi-table writes
- board detail pages with high join density

#### Output to Capture
- current plan
- rows scanned
- index usage
- worst-case latency
- recommendation per query

#### Definition of Done
- ada baseline perf notes yang terdokumentasi
- index/query changes dilakukan berdasarkan evidence

---

### P1.5 — Cursor / Pagination Strategy

#### Goal
Pastikan endpoint feed/ops aman saat data mulai besar.

#### Planned Work
- migrate feed-ish endpoints dari hard limit biasa ke cursor-friendly pagination bila perlu
- especially for:
  - activity feed
  - ops events
  - runs
  - notifications
- define stable sort keys

#### Definition of Done
- feed endpoints tidak degrade brutal saat data tumbuh

---

## P2 — Security & Policy Model

---

### P2.1 — Granular Action Permissions

#### Goal
Backend ngerti capability per action, bukan cuma “boleh akses board atau nggak”.

#### Planned Work
Tambahkan policy per action seperti:
- can_comment
- can_update_status
- can_request_review
- can_approve_review
- can_access_docs
- can_create_deliverables
- can_manage_connector

#### Scope
- users
- board members
- agents
- connectors/runtime identities

#### Definition of Done
- dangerous/sensitive actions guarded by explicit capability checks

---

### P2.2 — Runtime Policy Layer

#### Goal
Pisahkan dengan lebih jelas:
- identity
- connector auth
- board membership
- runtime action capability

#### Planned Work
- harden machine-token scope semantics
- support connector-specific restrictions
- validate runtime actor against board policy before action
- clearer forbidden vs unauthorized responses

#### Definition of Done
- runtime access model lebih robust untuk multi-runtime future

---

## P2 — Domain Quality Upgrades

---

### P2.3 — Canonical Signal Layer for Attention/Activity/Health

#### Goal
Kurangi stitching logic yang tersebar di query SQL campuran.

#### Problem Today
Activity/attention/health sudah useful, tapi semantiknya masih campuran antara raw query logic dan business meaning.

#### Planned Work
- define canonical signal/event meaning layer
- optional signal materialization or normalized derived tables
- unify naming and severity semantics
- make health/attention more reusable outside homepage

#### Definition of Done
- signal model reusable untuk homepage, board detail, ops center, notifications

---

### P2.4 — Event Schema Cleanup

#### Goal
Biar queue/delivery/events nggak makin kusut saat use case bertambah.

#### Planned Work
- standardize `event_type`
- standardize payload schema versioning
- normalize response error storage
- define retry classification:
  - retryable
  - permanent
  - dead
- improve dead-letter handling policy

#### Definition of Done
- event delivery pipeline lebih predictable dan easier to debug

---

# Suggested Execution Order

## Sprint A — Integrity First
Focus:
- P0.1 Formal task state machine
- P0.2 Review decision model
- P0.3 task/run/agent consistency guard

Why first:
- ini paling besar dampaknya buat data correctness
- semua observability dan maintenance lebih enak kalau state semantics udah benar dulu

---

## Sprint B — Maintenance & Audit
Focus:
- P1.1 structured audit log
- P1.2 background maintenance jobs
- P0.4 health/readiness contract

Why next:
- setelah lifecycle rapi, kita bikin sistem tahan banting dan bisa dirawat

---

## Sprint C — Measured Performance
Focus:
- P1.3 ops metrics
- P1.4 `EXPLAIN ANALYZE` pass
- P1.5 pagination strategy

Why next:
- optimization paling bagus dilakukan setelah workflow dan maintenance stabil

---

## Sprint D — Security & Future Runtime Scale
Focus:
- P2.1 granular permissions
- P2.2 runtime policy layer
- P2.4 event schema cleanup

Why next:
- penting saat agent/runtime makin banyak dan external connector makin bervariasi

---

## Sprint E — Signal Architecture Cleanup
Focus:
- P2.3 canonical signal layer

Why later:
- high leverage, tapi bagus dikerjakan setelah lifecycle, maintenance, dan perf baseline lebih stabil

---

## P3 — Future Inter-Agent Collaboration / A2A Compatibility

### P3.1 — A2A-Compatible Collaboration Layer

#### Goal
Siapin MoziBoard untuk masa depan di mana agent bisa saling delegasi/collaborate dengan semantics yang lebih native, tanpa bikin backend sekarang jadi terlalu kompleks terlalu cepat.

#### Product Positioning
MoziBoard **tidak perlu berubah jadi peer-to-peer agent mesh**.
Arah yang lebih masuk akal:
- **MoziBoard tetap jadi mission control / orchestration hub**
- tapi internal model dan connector layer disiapkan supaya nanti bisa support **A2A-style collaboration semantics**
- artinya agent bisa “berinteraksi dengan agent lain” secara logical, walau transport/runtime flow tetap dimediasi hub untuk audit, policy, routing, dan debugging

#### Why Later, Not Now
A2A bakal jauh lebih aman dan berguna setelah fondasi ini matang dulu:
- task state machine
- review semantics
- task/run/agent consistency guard
- audit log
- permissions/runtime policy

Kalau dipaksain terlalu awal, risiko utamanya:
- task delegation chaos
- susah audit siapa nyuruh siapa
- state drift makin parah
- debugging & trust boundary makin rumit

#### Planned Future Capabilities
Saat backend core sudah stabil, pertimbangkan nambah:
- capability manifest per agent / connector
- structured delegation contract:
  - intent
  - context
  - constraints
  - expected output
  - deadline / priority
- task handoff / subtask delegation flow antar agent
- inter-agent request / response event schema
- optional mediated inbox/outbox antar agent via MoziBoard
- capability discovery (minimal scoped, bukan open network discovery)
- compatibility adapter untuk runtime/protocol eksternal bila relevan
  - mis. webhook/native runtime/A2A-style bridge/MCP-adjacent bridge

#### Non-Goal
- bukan bikin semua agent bebas konek peer-to-peer secara liar
- bukan discovery global tanpa trust boundary
- bukan prioritas sebelum workflow integrity matang

#### Definition of Done (Future Phase)
- agent bisa mendelegasikan kerja ke agent lain lewat contract yang terstruktur
- semua delegation/handoff tercatat rapi
- permission + audit + reconciliation tetap jalan
- MoziBoard tetap jadi source of truth untuk coordination state

#### Recommended Timing
Pertimbangkan setelah:
- Sprint A selesai
- Sprint B audit/maintenance selesai
- Sprint D security/runtime policy cukup matang

Practical milestone:
- treat as **post-core roadmap / after backend integrity phase**, not current sprint

---

# Recommended Immediate Next Sprint

Kalau cuma boleh pilih satu sprint setelah reset, gw rekomendasiin ini:

## **Backend Improvement Sprint Next**
1. formal task state machine
2. review decision model
3. task/run/agent consistency guard
4. minimal audit log for important status/assignment/review actions

### Why this is the best next move
Karena ini langsung menaikkan:
- correctness
- debuggability
- confidence saat agent runtime makin aktif
- readiness buat maintenance jobs dan permission hardening sesudahnya

---

# Suggested Deliverable Format for Next Session

Kalau mau eksekusi roadmap ini setelah reset, mulai dengan urutan:

1. read this doc
2. inspect current task/runtime/agent status update paths
3. design transition matrix first
4. implement validation layer
5. patch all write paths to use it
6. add tests/logging
7. Docker build + restart + smoke

---

# Short Reset Resume Prompt

Kalau habis reset, pakai prompt ini:

> Lanjut MoziBoard backend improvement roadmap. Fokus Sprint A: formal task state machine, review decision model, dan consistency guard antara task/agent_runs/agents. Baca `docs/backend-improvement-roadmap-2026-03.md` dulu lalu eksekusi plan-nya.
