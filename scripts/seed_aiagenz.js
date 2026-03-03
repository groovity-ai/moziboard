const axios = require('axios');

const BOARD_ID = "193e6662-c16a-4ebd-b592-9d762f084fee";
const API_URL = "http://localhost:8080/api/tasks";

const tasks = [
    // Backlog
    { list_id: "backlog", title: "[Feature] No-Code Agent Builder", description: "Drag-and-drop UI for users to create agents." },
    { list_id: "backlog", title: "[Feature] Knowledge Hub (RAG)", description: "User upload PDF/SOP for agent knowledge base." },
    { list_id: "backlog", title: "[Feature] Multi-Channel Integration", description: "Connect agent to WA, Telegram, Discord." },
    { list_id: "backlog", title: "[Business] Affiliate System", description: "Commission for user referrals." },
    { list_id: "backlog", title: "[Concept] A2A Economy Protocol", description: "Standard API for agent-to-agent transactions." },

    // To Do (MVP)
    { list_id: "todo", title: "[Infra] Setup Server Production", description: "Install Docker + gVisor, Setup Traefik/Nginx Proxy Manager." },
    { list_id: "todo", title: "[Agent] CuanBot Monolith Container", description: "Merge Code Agent + Backend, Dockerfile Multi-Stage." },
    { list_id: "todo", title: "[Backend] API Manager", description: "Node.js/Go API for deploy/stop container." },
    { list_id: "todo", title: "[Frontend] Web Dashboard", description: "Next.js Dashboard: Login, List Agent, Deploy Button." },

    // Done
    { list_id: "done", title: "[Concept] Finalize Name: AiAgenz", description: "Marketplace & SaaS Platform." },
    { list_id: "done", title: "[Concept] Business Model", description: "SaaS Subscription + Token Economy." },
    { list_id: "done", title: "[Docs] Write PRD", description: "Detailed Product Requirement Document created." },
    { list_id: "done", title: "[Docs] Write A2A Concept", description: "Agent Economy concept paper created." }
];

async function seed() {
    for (const t of tasks) {
        try {
            await axios.post(API_URL, {
                board_id: BOARD_ID,
                title: t.title,
                description: t.description,
                list_id: t.list_id
            });
            console.log(`Created: ${t.title}`);
        } catch (e) {
            console.error(`Failed: ${t.title}`, e.message);
        }
    }
}

seed();
