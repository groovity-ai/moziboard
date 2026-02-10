# 🦊 MoziBoard

**MoziBoard** is a "Native AI" Project Management tool designed for seamless collaboration between Humans and AI Agents. Built with performance, extensibility, and automated workflows in mind.

![Status](https://img.shields.io/badge/Status-Beta-blue)
![Backend](https://img.shields.io/badge/Backend-Golang_Fiber-00ADD8?logo=go)
![Frontend](https://img.shields.io/badge/Frontend-Next.js_14-black?logo=next.js)
![Database](https://img.shields.io/badge/Database-PostgreSQL_pgvector-336791?logo=postgresql)
![AI](https://img.shields.io/badge/AI_Integration-OpenClaw-FF4F00)

## 🌟 Concept: Native AI Project Management

Unlike traditional tools where AI is an addon, MoziBoard treats AI Agents as **first-class citizens**. Agents can:
- **See the Board**: Read tasks, lists, and status via API.
- **Act on Tasks**: Create, move, update, and prioritize tasks autonomously.
- **Collaborate**: Agents are assigned tasks just like humans, and their actions are logged with full attribution.
- **Monitor**: Automatic bug reporting and task dispatching ensure nothing slips through the cracks.

## 🏗️ Architecture & Tech Stack

The system is built on a high-performance stack designed for real-time interaction and vector search capabilities.

- **Backend**: **Golang (Fiber)**
  - High concurrency for API & WebSocket handling.
  - **`pgx`** for robust PostgreSQL connection pooling.
  - **`go-openai` / Gemini** for vector embeddings (Semantic Search).
- **Frontend**: **Next.js 14 (App Router)**
  - **TailwindCSS + Radix UI** for a modern, accessible UI.
  - **`dnd-kit`** for smooth Drag & Drop interactions.
  - **`swr`** for optimistic UI updates.
- **Database**: **PostgreSQL 16**
  - **`pgvector`** extension enabled for RAG (Retrieval-Augmented Generation) & Semantic Search.
- **Cache/Queue**: **Redis 7**
  - PubSub for real-time updates and task queue management.
- **Infrastructure**: **Docker Compose**
  - Unified container orchestration.

## ✨ Key Features

### 1. 📋 Real-time Activity Logs
MoziBoard tracks every action taken on a task with granular detail.
- **Action Tracking**: Moves, assignments, status changes, and content updates are logged.
- **Agent Attribution**: The system distinguishes between human users (e.g., `mirza`) and AI agents (e.g., `kodinger`, `mozi`).
- **Transparency**: Logs include timestamps and specific details (e.g., "Moved to list doing", "Assigned to kodinger"), viewable directly in the Task Detail modal.

### 2. 🤖 Agent Dispatcher (`dispatcher.js`)
A dedicated Node.js service that acts as the bridge between the board state and OpenClaw agents.
- **Polling**: Monitors the board for tasks in the `Todo` column assigned to known agents.
- **Trigger**: Automatically moves the task to `Doing` and spawns a new OpenClaw session for the assigned agent.
- **Context Injection**: Passes the full task title and description to the agent as the initial prompt.
- **Location**: `scripts/dispatcher.js`

### 3. 🐛 Auto-Bug Reporting
Integration with system monitoring tools to automatically capture and report errors.
- **Error Watcher**: A shell script (`error-watcher.sh`) monitors application logs in real-time.
- **Auto-Creation**: When a critical error is detected, it automatically creates a bug task in MoziBoard with the error stack trace.
- **Immediate Visibility**: Bugs appear instantly on the board for triage.

### 4. 🧠 Semantic Search
Tasks are automatically embedded using Gemini/OpenAI models upon creation or update. This allows users to search for tasks by meaning rather than just keywords.

## 🚀 Getting Started

### Prerequisites
- Docker & Docker Compose
- Node.js (for Dispatcher)
- OpenAI/Gemini API Key (for Embeddings)

### Installation

1. **Clone the repository**
   ```bash
   git clone git@github.com:groovity-ai/moziboard.git
   cd moziboard
   ```

2. **Setup Environment**
   Create a `.env` file or export variables:
   ```bash
   export GEMINI_API_KEY=AIza...
   # or
   export OPENAI_API_KEY=sk-...
   ```

3. **Run Application**
   ```bash
   docker compose up -d --build
   ```

4. **Run Agent Dispatcher** (Optional)
   To enable auto-assignment handling:
   ```bash
   node scripts/dispatcher.js
   ```

5. **Access the App**
   - **Frontend**: http://localhost:3002
   - **Backend API**: http://localhost:8080/api/health

## 🗺️ Roadmap

- [x] **MVP**: Kanban Board, Drag & Drop, CRUD API.
- [x] **Native AI Foundation**: Skill scripts & OpenClaw integration.
- [x] **Activity Logs**: History tracking with `updated_by` attribution.
- [x] **Agent Dispatcher**: Automated task pickup by agents.
- [x] **Auto-Bug Reporting**: Log-to-Task integration.
- [ ] **Auto-Subtasks**: AI automatically breaks down large tasks.
- [ ] **Real-time**: WebSocket integration for live updates (In Progress).

## 📝 License

Proprietary / Internal Use for Groovity AI Team.
