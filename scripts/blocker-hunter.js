const axios = require('axios');
const { exec } = require('child_process');

const BASE_URL = 'http://localhost:8080/api';
const REPORT_TO_ID = '41434457'; // Telegram ID
const STALE_THRESHOLD_HOURS = 24;

async function runCommand(command) {
    return new Promise((resolve, reject) => {
        exec(command, (error, stdout, stderr) => {
            if (error) {
                console.warn(`Command failed: ${command}`, stderr);
                resolve(null);
            } else {
                resolve(stdout.trim());
            }
        });
    });
}

async function checkBlockers() {
    try {
        console.log('🔍 Hunting for blockers...');
        
        // 1. Fetch Boards
        const boardsRes = await axios.get(`${BASE_URL}/boards`);
        const boards = boardsRes.data;
        
        let staleTasks = [];

        for (const board of boards) {
            // 2. Fetch Tasks
            const tasksRes = await axios.get(`${BASE_URL}/boards/${board.id}/tasks`);
            const tasks = tasksRes.data;

            // Filter 'doing'
            const doingTasks = tasks.filter(t => t.list_id === 'doing');

            for (const task of doingTasks) {
                // 3. Fetch Activities
                let lastActivityTime = null;
                try {
                    const actRes = await axios.get(`${BASE_URL}/tasks/${task.id}/activities`);
                    const activities = actRes.data; // Sorted by created_at DESC from backend

                    if (activities && activities.length > 0) {
                        lastActivityTime = new Date(activities[0].created_at);
                    }
                } catch (e) {
                    console.error(`Failed to fetch activities for task ${task.id}`);
                }

                // 4. Check Stale Status
                let isStale = false;
                let hoursStuck = 0;
                
                if (lastActivityTime) {
                    const diffMs = new Date() - lastActivityTime;
                    const diffHours = diffMs / (1000 * 60 * 60);
                    if (diffHours > STALE_THRESHOLD_HOURS) {
                        isStale = true;
                        hoursStuck = Math.floor(diffHours);
                    }
                } else {
                    // No activity found - Assume OLD/Stale (created > 24h ago or before logging)
                    isStale = true;
                    hoursStuck = "24+ (No logs)";
                }

                if (isStale) {
                    staleTasks.push({
                        board: board.title,
                        title: task.title,
                        assignee: task.assignee_id || 'Unassigned',
                        hours: hoursStuck
                    });
                }
            }
        }

        // 5. Report
        if (staleTasks.length > 0) {
            console.log(`⚠️ Found ${staleTasks.length} stale tasks.`);
            
            let message = "🚨 **MoziBoard Blocker Report**\n\nThe following tasks have been stuck in 'Doing' for >24 hours:\n\n";
            staleTasks.forEach(t => {
                message += `• **${t.title}** (${t.assignee})\n   Board: ${t.board} | Stuck: ${t.hours}h\n`;
            });

            console.log('Sending report via OpenClaw...');
            // Escape special chars for shell command
            const safeMessage = message.replace(/"/g, '\\"').replace(/`/g, '\\`').replace(/\$/g, '\\$');
            
            await runCommand(`openclaw message send --target telegram:${REPORT_TO_ID} --message "${safeMessage}"`);
            console.log('✅ Report sent.');
        } else {
            console.log('✅ No blockers found! Team is moving fast.');
        }

    } catch (error) {
        console.error('Error in blocker-hunter:', error.message);
    }
}

checkBlockers();
