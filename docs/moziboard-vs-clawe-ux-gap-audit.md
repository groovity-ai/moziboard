# MoziBoard vs Clawe UX Gap Audit

Last updated: 2026-03-10

This document audits MoziBoard's current Kanban and task-detail UX against the local Clawe codebase checked on the server.

The goal is not to copy Clawe blindly, but to identify where Clawe currently sets a stronger quality bar in:
- interaction confidence
- information architecture
- visual hierarchy
- board ergonomics
- task detail usability

This audit is grounded in code inspection of both repositories.

---

# 1. Scope

## MoziBoard files reviewed
- `frontend/src/components/Board.tsx`
- `frontend/src/components/ListContainer.tsx`
- `frontend/src/components/TaskCard.tsx`
- `frontend/src/components/TaskDetailModal.tsx`

## Clawe files reviewed
- `apps/web/src/components/kanban/kanban-board.tsx`
- `apps/web/src/components/kanban/kanban-column.tsx`
- `apps/web/src/components/kanban/kanban-card.tsx`
- `apps/web/src/app/(dashboard)/board/page.tsx`

---

# 2. Executive Summary

## Bottom line
MoziBoard currently has more ambitious interaction intent than Clawe in one narrow area:
- drag and drop

But the interaction is not yet trustworthy enough.

Clawe, by contrast, feels more polished because it is:
- more disciplined about UX scope
- stronger in board layout architecture
- better at column semantics
- better at task card information density
- better at panel-oriented workspace design

### Verdict
MoziBoard currently feels like:
- a functional internal tool with unfinished interaction layers

Clawe currently feels like:
- a more cohesive workspace product with stronger operational readability

The biggest issue is not just styling.
The main gap is that MoziBoard's **core task workflow interaction layer does not yet feel trustworthy or intentional**.

---

# 3. Key Findings

# 3.1 Drag and Drop: MoziBoard is More Ambitious but Less Reliable

## MoziBoard
MoziBoard uses `dnd-kit` in `Board.tsx` and attempts true Kanban movement via:
- `DndContext`
- `SortableContext`
- `TaskCard` sortable items
- optimistic updates through SWR mutate

However, the current implementation mainly updates:
- `list_id`

and does not appear to fully handle:
- stable reorder inside the same list
- precise insertion target logic
- robust position recalculation on drop
- explicit drag handle vs click separation

### User-facing consequence
The UI can feel like:
- drag technically exists
- but drop intent is not visually or behaviorally trustworthy
- cards may feel jumpy or ambiguous
- click-to-open and drag-to-move are not strongly separated

## Clawe
Clawe's current Kanban implementation is much more conservative.
From the reviewed code, the board is:
- horizontally scrollable
- strongly structured
- card-click focused
- not selling a premium DnD interaction in the visible layer reviewed

### Why Clawe wins here
Clawe avoids promising a high-fidelity interaction it has not clearly committed to in the reviewed UI layer.
That gives it a more confident feel.

## Gap assessment
### MoziBoard weakness
MoziBoard is trying to offer a premium interaction without enough support logic or affordance.

### Recommendation
MoziBoard should either:
1. fully commit to a premium Kanban drag interaction, or
2. temporarily reduce DnD expectations until the interaction is stable

The current middle state is UX-negative.

---

# 3.2 Board Layout Architecture: Clawe Feels Like a Workspace, MoziBoard Feels Like a Page

## MoziBoard
MoziBoard board layout currently centers on:
- search bar
- horizontal list strip
- modal detail

This is workable, but it still feels page-like.

## Clawe
Clawe's board page uses a stronger workspace layout:
- `ResizablePanelGroup`
- left-side `AgentsPanel`
- central board area
- `LiveFeed` opened through drawer
- board wrapped in a more desktop-like dashboard structure

### Why this matters
Clawe's board is not just a kanban.
It feels like part of an operating environment.

## Gap assessment
### MoziBoard weakness
MoziBoard still feels like a board feature inside an app.
Clawe feels like a persistent control surface.

### Recommendation
MoziBoard board UX should evolve toward:
- multi-panel workspace thinking
- stronger persistent side context
- less reliance on centered modal interruption

---

# 3.3 Column Semantics: Clawe Makes Workflow Stages Legible

## MoziBoard
Current `ListContainer.tsx` renders columns with:
- title
- count badge
- plain container styling

Visually, columns are mostly equivalent.

## Clawe
Clawe's `kanban-column.tsx` gives columns stronger semantic identity through:
- per-stage icons
- per-stage visual variants
- stronger badge treatment
- intentional empty states
- internal task-list scroll area

### Why this matters
A kanban board is not only a set of containers.
It is a workflow map.
Clawe communicates workflow semantics better.

## Gap assessment
### MoziBoard weakness
Columns are functionally present, but not semantically expressive.

### Recommendation
MoziBoard should add:
- list-stage visual variants
- stage icons
- stronger headers
- meaningful empty states
- clearer internal scroll behavior per column

---

# 3.4 Task Cards: Clawe Delivers Better Signal Density

## MoziBoard
Current task cards show:
- title
- short description preview
- assignee avatar
- drag grip icon on hover

This keeps cards visually simple, but they are too information-thin for an operational board.

## Clawe
Clawe's `kanban-card.tsx` includes:
- title
- description preview
- expandable/full description via popover
- priority badge
- assignee text
- subtask summary
- document count
- expandable subtasks
- subtask status indicators

### Why this matters
Clawe cards carry more decision-making signal without becoming noisy.

## Gap assessment
### MoziBoard weakness
Cards are too sparse to support fast board scanning.

### Recommendation
MoziBoard cards should evolve to show some combination of:
- priority
- review/blocker state
- comment count
- deliverable count
- task type or source badge
- clearer assignee signal
- optional subtask progress

---

# 3.5 Click vs Drag Affordance: MoziBoard is Ambiguous

## MoziBoard
In `TaskCard.tsx`, the entire card receives sortable listeners.
This creates a UX ambiguity between:
- click to open details
- drag to move task

The small grip icon is mostly decorative in the current behavior pattern.

## Clawe
Clawe cards are clearly click-focused in the reviewed implementation.
There is less ambiguity about what the card is meant to do.

## Gap assessment
### MoziBoard weakness
The interaction contract is unclear.

### Recommendation
If MoziBoard keeps DnD:
- move drag listeners to an explicit drag handle
- let card body remain click-first
- make hover/active/drop states much more explicit

---

# 3.6 Task Detail UX: MoziBoard is Still a CRUD Modal

## MoziBoard
`TaskDetailModal.tsx` currently provides:
- modal header
- two tabs (`discussion`, `details`)
- comments thread
- assignee selector
- board selector
- description editor
- deliverables section
- activity section

This is useful, but it still feels like a collection of fields and panels inside a modal.

### Main UX issues
- too much stacked vertical content
- weak top-level summary
- discussion is basic chat styling rather than operational conversation design
- activity and deliverables feel attached rather than integrated
- editing and review do not feel like a focused workspace

## Clawe
From the board architecture and adjacent panel systems, Clawe is clearly moving toward:
- panel-oriented interaction
- workspace continuity
- lower interruption cost

Even without reviewing the full task detail modal implementation, the surrounding structure indicates a more mature design direction.

## Gap assessment
### MoziBoard weakness
Task detail is still a popup form experience instead of a task workspace.

### Recommendation
MoziBoard should move toward a side panel or right drawer pattern for task detail with:
- summary header
- clearer metadata grouping
- discussion as a first-class pane
- deliverables and activity as secondary but readable sections

---

# 3.7 Empty States and Board Confidence

## MoziBoard
Empty task lists and empty deliverable/activity sections currently feel utilitarian.

## Clawe
Clawe uses stronger empty-state treatment with:
- icons
- consistent spacing
- lighter semantic messaging

## Gap assessment
### MoziBoard weakness
The board feels unfinished when empty.

### Recommendation
Improve empty states across:
- columns
- task discussion
- deliverables
- activity

This will increase perceived product maturity without major backend work.

---

# 4. Why Clawe Currently Feels Better

Clawe does not necessarily win because it has more complex behavior.
It wins because it is more disciplined in four areas:

## 4.1 Scope discipline
Clawe does not over-promise interaction complexity in the reviewed Kanban layer.

## 4.2 Semantic styling
Columns, cards, and layout encode workflow meaning better.

## 4.3 Workspace architecture
Clawe uses panels, drawers, and resizable layout to create a stronger operational shell.

## 4.4 Signal density
Clawe's cards and columns expose more useful information without turning into visual clutter.

---

# 5. MoziBoard Design Direction Recommendation

MoziBoard should not merely copy Clawe.
It should adopt the principles that make Clawe stronger while preserving MoziBoard's own mission-control direction.

## Desired MoziBoard direction
- stronger operational scanability
- trustworthy DnD interactions
- richer card signals
- task detail as a workspace, not a popup form
- board layout that feels like an agent command center

---

# 6. Priority Fix Plan

# P0 — Fix the interaction spine

## 6.1 Rebuild drag and drop properly
Required improvements:
- stable reorder inside a list
- precise insertion target behavior
- position recalculation on drop
- explicit drag handle
- click body separated from drag gesture
- stronger optimistic UI reconciliation

## Why first
Because untrustworthy DnD damages confidence more than plain visuals do.

---

## 6.2 Redesign task detail into a right-side workspace panel
Recommended structure:
- summary header
- overview section
- discussion tab
- deliverables tab
- activity tab
- metadata section

## Why first
Because task detail is where users spend focused time after scan.
The current modal structure is too CRUD-like.

---

# P1 — Increase board readability

## 6.3 Add semantic column styling
- stage icons
- stage variants
- better empty states
- more intentional count display

## 6.4 Enrich task cards
Suggested additions:
- priority badge
- assignee label or clearer avatar treatment
- comments count
- deliverables count
- blocker/review state
- optional subtask progress

---

# P2 — Polish and confidence

## 6.5 Improve animation and feedback
- smoother hover states
- stronger drop indicators
- save/update confirmation feedback
- visual sync/realtime confidence

## 6.6 Improve detail ergonomics
- cleaner composer
- more readable activity timeline
- stronger distinction between human, agent, and system actions

---

# 7. Proposed Implementation Sequence

## Phase 1
- redesign DnD logic
- explicit drag handle
- stronger card interaction model

## Phase 2
- refactor task detail from modal-heavy CRUD to side-panel workspace
- add summary header and grouped sections

## Phase 3
- add semantic column styles and richer card signals
- improve empty states

## Phase 4
- motion polish, sync feedback, and high-confidence refinements

---

# 8. Final Verdict

If the benchmark is Clawe, then MoziBoard currently lags behind in:
- interaction confidence
- task board readability
- task detail ergonomics
- workspace architecture maturity

MoziBoard's current board experience is not irredeemable.
The core issue is that it is halfway between:
- internal CRUD tool
- premium mission-control workspace

To catch up with the Clawe quality bar, MoziBoard must stop applying surface-level polish first and instead fix:
1. the interaction spine
2. the board/task information architecture
3. the workspace layout model

That is the clearest path from "functional board" to "credible operational product."
