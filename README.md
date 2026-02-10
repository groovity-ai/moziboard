# 🦊 Moziboard

**Moziboard** is a "Native AI" Project Management tool designed for seamless collaboration between Humans and AI Agents. Built with performance and extensibility in mind.

![Moziboard Status](https://img.shields.io/badge/Status-Beta-blue)
![Backend](https://img.shields.io/badge/Backend-Golang_1.23-00ADD8?logo=go)
![Frontend](https://img.shields.io/badge/Frontend-Next.js_14-black?logo=next.js)
![Database](https://img.shields.io/badge/Database-PostgreSQL_pgvector-336791?logo=postgresql)
![AI](https://img.shields.io/badge/AI_Integration-OpenClaw-FF4F00)

## 🌟 Concept: Native AI Project Management

Unlike traditional tools (Trello, Jira) where AI is an addon, Moziboard treats AI Agents as **first-class citizens**. Agents can:
- **See the Board**: Read tasks, lists, and status via API.
- **Act on Tasks**: Create, move, update, and prioritize tasks autonomously.
- **Contextualize**: Append chat summaries directly to task descriptions ("Chat-to-Context").
- **Monitor**: Run cron jobs to detect blockers or stale tasks.

## 🏗️ Tech Stack

- **Backend**: Golang (Fiber)
  - High concurrency for WebSocket & AI orchestration.
  - `pgx` for PostgreSQL connection pooling.
  - `go-openai` for vector embeddings (Semantic Search).
- **Frontend**: Next.js 14 (App Router)
  - TailwindCSS + Radix UI for styling.
  - `dnd-kit` for smooth Drag & Drop.
  - `swr` for real-time data fetching (Optimistic UI).
- **Database**: PostgreSQL 16
  - `pgvector` extension enabled for Semantic Search (RAG).
- **Cache/Queue**: Redis 7
  - Task queue for background AI processing.
- **Infrastructure**: Docker Compose
  - One command to run everything.

## 🚀 Getting Started

### Prerequisites
- Docker & Docker Compose
- OpenAI API Key (Optional, for Semantic Search)

### Installation

1. **Clone the repository**
   ```bash
   git clone git@github.com:groovity-ai/moziboard.git
   cd moziboard
   ```

2. **Setup Environment**
   Set your API Key (Choose one):
   
   **Option A: Google Gemini (Recommended & Free)**
   ```bash
   # Get key from aistudio.google.com
   export GEMINI_API_KEY=AIza...
   ```

   **Option B: OpenAI**
   ```bash
   export OPENAI_API_KEY=sk-...
   ```

3. **Run with Docker Compose**
   ```bash
   docker compose up -d --build
   ```

4. **Access the App**
   - **Frontend**: http://localhost:3002
   - **Backend API**: http://localhost:8080/api/health

## 🤖 AI Agent Integration (OpenClaw)

To enable your AI Agent (e.g., OpenClaw) to manage the board, install the **Moziboard Skill**.

### Skill Scripts
Located in `skills/moziboard/scripts/`:
- `create-task.sh`: Create a new task.
- `list-tasks.sh`: List all tasks.
- `update-task.sh`: Move or update a task.
- `append-context.sh`: Append chat history/notes to a task description.
- `check-blockers.sh`: Identify tasks stuck in "In Progress".

### Agent Capabilities
Once equipped, you can ask your agent:
- *"Create a task for redesigning the homepage."*
- *"Move task #4 to Done."*
- *"Add a note to task #2 that we decided to use Tailwind."* (Chat-to-Context)
- *"Are there any blockers today?"* (Runs `check-blockers.sh`)

## 🗺️ Roadmap

- [x] **MVP**: Kanban Board, Drag & Drop, CRUD API.
- [x] **Native AI Foundation**: Skill scripts & OpenClaw integration.
- [x] **Context Injection**: Chat-to-Context feature.
- [ ] **Semantic Search**: Search tasks by meaning (Backend ready, need API Key).
- [ ] **Auto-Subtasks**: AI automatically breaks down large tasks.
- [ ] **Real-time**: WebSocket integration for live updates.

## 📝 License

Proprietary / Internal Use for Groovity AI Team.
