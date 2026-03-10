# MoziBoard Board Redesign P0 Plan

Last updated: 2026-03-10

This document defines the first implementation slice for improving MoziBoard board UX after the Clawe comparison audit.

P0 focuses on the **interaction spine**, not full visual redesign.

---

# Goal

Make the Kanban board feel trustworthy again by fixing the most damaging UX issue first:

- drag and drop should be stable, predictable, and visually understandable

This phase does **not** aim to fully redesign task detail yet.

---

# P0 Scope

## In scope
- explicit drag handle on task cards
- click body opens task detail
- stable reorder inside the same list
- stable move across lists
- local optimistic state that reflects drop position immediately
- task `position` recalculation on drop
- basic drop-target highlighting on active list

## Out of scope
- full task detail redesign
- richer card metadata
- column semantic redesign
- task detail side panel migration
- keyboard-heavy accessibility polish beyond current DnD support

---

# Desired User Experience

## Card interaction contract
- click task body → open task detail
- drag handle only → move task
- no more whole-card drag ambiguity

## Drop behavior
- dragging over another task should indicate insertion intent
- dropping inside the same list should reorder predictably
- dropping into a different list should move task and assign a correct position
- UI should update immediately after drop, before websocket refresh

## Recovery behavior
- if API update fails, rollback to server state

---

# Data Model Expectations

Current task fields already available:
- `id`
- `board_id`
- `list_id`
- `position`

P0 assumes backend `PUT /api/tasks/:id` accepts updated `list_id` and `position`.

If multiple tasks must be persisted after a reorder, frontend may temporarily issue multiple `PUT`s until a bulk reorder endpoint exists.

---

# Implementation Plan

## 1. Board state helpers
Add helper functions in `Board.tsx` (or extracted local utility later) to:
- find source list + task index
- find target list + task index
- move task within the same list
- move task across lists
- recalculate positions after any move
- flatten lists back into task updates for optimistic state

## 2. Explicit drag handle
Update `TaskCard.tsx` so:
- `useSortable` still owns card container
- sortable `listeners` are attached only to the drag handle button
- card body retains `onClick`
- drag handle gets visible affordance even before hover

## 3. Better list droppable state
Update `ListContainer.tsx` so:
- active drop list gets stronger border/background state
- empty list still presents a clear drop zone

## 4. Better drag end logic
Replace current simplistic `handleDragEnd` with logic that:
- handles same-list reordering
- handles cross-list move + insertion
- recalculates `position` values
- updates optimistic SWR state immediately
- persists changed tasks
- rolls back with SWR revalidation on failure

---

# Acceptance Criteria

## Interaction
- user can drag a task to reorder within the same column
- user can drag a task into another column at a specific spot
- user can click card body without accidentally dragging
- drag handle is clearly discoverable

## Visual
- active drop list visibly highlights
- drag overlay still appears
- insertion outcome feels immediate after drop

## Data
- task order persists after refresh
- task list changes persist after refresh
- failed update reverts cleanly

---

# Files Expected to Change

## Primary
- `frontend/src/components/Board.tsx`
- `frontend/src/components/TaskCard.tsx`
- `frontend/src/components/ListContainer.tsx`

## Optional follow-up
- extracted utility file for task move/reorder helpers

---

# Follow-up After P0

After the DnD spine is stable, next phases should be:
1. task detail redesign
2. card signal enrichment
3. semantic column styling
4. board/workspace architecture polish

---

# Success Definition

P0 is successful if the board no longer feels broken or ambiguous during the most common interaction:

> picking up a task, moving it, dropping it, and trusting the result.
