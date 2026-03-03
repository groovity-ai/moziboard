const axios = require('axios');
const fs = require('fs');
const path = require('path');

const BOARD_ID = "193e6662-c16a-4ebd-b592-9d762f084fee";
const API_URL = `http://localhost:8080/api/boards/${BOARD_ID}/docs`;

const docs = [
    {
        title: "Product Requirement Document (PRD) - AiAgenz",
        file: "AiAgenz_PRD.md"
    },
    {
        title: "Concept - Agent-to-Agent (A2A) Economy",
        file: "AiAgenz_A2A_Economy.md"
    }
];

async function seedDocs() {
    for (const d of docs) {
        try {
            const content = fs.readFileSync(path.join(__dirname, '../docs', d.file), 'utf8');
            await axios.post(API_URL, {
                title: d.title,
                content: content
            });
            console.log(`Uploaded Doc: ${d.title}`);
        } catch (e) {
            console.error(`Failed Doc: ${d.title}`, e.message);
        }
    }
}

seedDocs();
