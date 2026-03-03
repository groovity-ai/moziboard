# Product Requirement Document (PRD) - AiAgenz

## 1. Executive Summary
**Product Name:** AiAgenz (SaaS AI Agent Marketplace & Hosting)  
**Tagline:** "Your Personal AI Workforce, One Click Away."  
**Mission:** Democratize access to autonomous AI agents for everyone (UMKM, traders, creators) through a simple, secure, and scalable platform.  
**Vision:** Build the world's first "Agent-to-Agent (A2A) Economy" where AI agents collaborate and transact autonomously.

## 2. Problem Statement
*   **Complexity:** Setting up AI agents (OpenClaw, AutoGPT) requires technical skills (Docker, VPS, API Keys) that 99% of people don't have.
*   **Security:** Running untrusted agent code on personal devices or shared servers is risky (malware, resource abuse).
*   **Monetization:** Developers have no easy way to distribute and monetize their custom agents without building their own SaaS.

## 3. Solution Overview
**AiAgenz** is a managed marketplace and hosting platform for AI Agents.
*   **For Users:** "App Store" experience. Browse -> Rent -> Click Start. No coding required.
*   **For Developers:** "Play Store" experience. Build -> Publish -> Earn Revenue (Subscription/Token).
*   **Infrastructure:** Secure sandboxed environment (Docker + gVisor) ensuring isolation and safety.

## 4. Key Features (MVP Phase)
### A. Marketplace (The Storefront)
*   **Agent Catalog:** Categories (CS, Trading, Content, Productivity).
*   **Search & Filter:** By popularity, price, rating.
*   **Agent Detail Page:** Description, pricing, user reviews, demo.

### B. User Dashboard (The Control Panel)
*   **My Agents:** List of rented agents.
*   **Control:** Start / Stop / Restart / Logs.
*   **Configuration:** Simple form inputs (Telegram Token, API Keys, Prompts).
*   **Billing:** Subscription status, credit usage.

### C. Developer Studio (The Factory)
*   **Upload Agent:** Submit Docker image or connect GitHub repo (later).
*   **Pricing Setup:** Set monthly fee or per-request token cost.
*   **Analytics:** Usage stats, revenue report.

### D. Infrastructure (The Engine)
*   **Sandboxing:** Google gVisor for kernel-level isolation.
*   **Resource Limits:** CPU/RAM quotas per plan.
*   **Networking:** Traefik/Nginx reverse proxy for automatic subdomains.
*   **One-Container Architecture:** Agent + Backend logic bundled in a single secure container.

## 5. Technical Architecture
*   **Frontend:** Next.js (React) - SEO friendly, fast.
*   **Backend:** Node.js / Go - Docker orchestration API.
*   **Database:** PostgreSQL (User/Agent data), Redis (Job queues), MongoDB (Chat logs).
*   **Container Runtime:** Docker Engine + gVisor (`runsc`).
*   **Reverse Proxy:** Traefik (Auto SSL & Routing).

## 6. Business Model
*   **Subscription (SaaS):** Monthly fee for hosting (Starter, Pro, Enterprise).
*   **Marketplace Commission:** % fee on every agent rental (e.g., 20% to Platform, 80% to Creator).
*   **Token Economy (Future A2A):** Transaction fees for agent-to-agent services.

## 7. Roadmap
### Phase 1: MVP (Month 1-2)
*   [ ] Secure Infrastructure Setup (VPS + gVisor).
*   [ ] "Monolith Container" Design for Internal Agents (CuanBot, etc.).
*   [ ] Basic Web Dashboard (Login, Deploy, Stop).
*   [ ] Payment Gateway Integration (Midtrans/Xendit).

### Phase 2: Marketplace (Month 3-4)
*   [ ] Public Marketplace UI.
*   [ ] Developer Onboarding & Submission Portal.
*   [ ] Billing System for Revenue Sharing.

### Phase 3: Ecosystem (Month 5+)
*   [ ] No-Code Agent Builder.
*   [ ] Agent-to-Agent (A2A) API Protocol.
*   [ ] Advanced Knowledge Base (RAG) Integration.

## 8. Success Metrics
*   **User Acquisition:** Number of registered users.
*   **Active Agents:** Number of running containers.
*   **MRR:** Monthly Recurring Revenue.
*   **Developer Engagement:** Number of published agents.
