# 📋 Board: AiAgenz (SaaS & Marketplace)

## 📌 Columns & Tasks

### 1. 🛑 Backlog (Ide & Fitur Masa Depan)
*   **[Feature] No-Code Agent Builder:** Drag-and-drop UI buat user bikin agent sendiri.
*   **[Feature] Knowledge Hub (RAG):** User upload PDF/SOP, agent langsung pinter.
*   **[Feature] Multi-Channel Integration:** Satu agent connect ke WA, Telegram, Discord sekaligus.
*   **[Business] Affiliate System:** User ajak temen dapet komisi.
*   **[Concept] A2A Economy Protocol:** Standar API buat agent saling transaksi/bayar.

### 2. 📝 To Do (MVP Phase - Prioritas Minggu Ini)
*   **[Infra] Setup Server Production:**
    *   Install Docker + gVisor (`runsc`) di VPS.
    *   Setup Traefik / Nginx Proxy Manager (Auto SSL & Subdomain).
*   **[Agent] CuanBot "Monolith" Container:**
    *   Gabungin Code Agent + Backend Logic jadi satu repo.
    *   Bikin `Dockerfile` Multi-Stage (Compile -> Runtime).
    *   Test deploy manual 1 container aman.
*   **[Backend] API Manager (Node.js/Go):**
    *   Endpoint `POST /deploy` (Spin up container user).
    *   Endpoint `POST /stop` & `GET /logs`.
*   **[Frontend] Web Dashboard (Next.js):**
    *   Login/Register User.
    *   List Agent Aktif.
    *   Tombol "Deploy CuanBot".

### 3. 🚧 In Progress
*   *Menyusun Dokumen PRD & Konsep (Done)*

### 4. ✅ Done
*   **[Concept] Finalize Name:** "AiAgenz" (Marketplace & SaaS).
*   **[Concept] Business Model:** Sewa Agent + Token Economy.
*   **[Docs] Write PRD:** `AiAgenz_PRD.md` created.
*   **[Docs] Write A2A Concept:** `AiAgenz_A2A_Economy.md` created.
