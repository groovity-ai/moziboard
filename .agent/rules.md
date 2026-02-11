# MoziBoard Agent Rules

These rules govern how the agent interacts with the project management system (MoziBoard) via MCP.

## 1. Task Management Automation

**Goal**: Ensure every significant piece of work is tracked in MoziBoard.

### When starting a new objective/task:
1. **Check Existing Tasks**: ALWAYS search for an existing task on the board using `mcp_mozi_list_tasks`.
   - If a relevant task exists (e.g., "Implement Feature X"), read its details.
   - If the task is in `todo`, move it to `doing` using `mcp_mozi_update_task`.

2. **Create New Task**: If NO relevant task exists for the current user objective:
   - Create a new task using `mcp_mozi_create_task`.
   - **Title**: Use the user's objective (e.g., "Fix Auth Bug", "Setup Docker").
   - **List**: Set to `doing` (since you are starting it now) or `todo`.
   - **Assignee**: Set to `antigravity` to indicate it is being handled by the IDE agent.
   - **Description**: Summarize the plan or the user's request.

### During execution:
- **Updates**: If the plan changes significantly or you discover new sub-tasks, update the task description.

### Upon completion:
- **Mark as Done**: When the user's objective is met and verified, move the task to the `done` list using `mcp_mozi_update_task`.

## 2. MCP Tool Usage
- Use `mcp_mozi_list_tasks` to see current board state.
- Use `mcp_mozi_create_task(title, description, list_id='todo')` for new work.
- Use `mcp_mozi_update_task(id, list_id='doing'|'done')` to track progress.

## 3. General Behavior
- **Be Proactive**: Don't wait for the user to ask you to update the board.
- **Context Awareness**: Use the task description to store relevant context or decisions made during the conversation.
