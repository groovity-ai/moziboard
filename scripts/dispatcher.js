const axios = require('axios');
const { spawn } = require('child_process');

const BASE_URL = 'http://localhost:8080/api';
// Agents list as per requirement (lowercase for normalized comparison)
const AGENTS = ['kodinger', 'devo', 'mozi', 'resepsionis', 'mimin'];
const POLL_INTERVAL = 60000; // 60 seconds

// Helper to execute shell commands
const runCommand = (command, args = []) => {
    return new Promise((resolve, reject) => {
        const proc = spawn(command, args, { stdio: ['pipe', 'pipe', 'pipe'] });
        let stdout = '';
        let stderr = '';
        proc.stdout.on('data', (data) => stdout += data);
        proc.stderr.on('data', (data) => stderr += data);
        proc.on('close', (code) => {
            if (code !== 0) {
                console.error(`Command failed: ${command} ${args.join(' ')}`, stderr);
                resolve(null);
            } else {
                resolve(stdout.trim());
            }
        });
    });
};

async function processTask(task) {
    const agentName = task.assignee_id;
    console.log(`Processing task ${task.id} for agent ${agentName}...`);

    try {
        // 1. Update task status
        // Ensure we send the full object with the updated status to avoid data loss
        const updatedTask = { ...task };

        // Update list_id to 'doing'
        updatedTask.list_id = 'doing';
        updatedTask.updated_by = agentName;

        // Send PUT request to /api/tasks/:id
        await axios.put(`${BASE_URL}/tasks/${task.id}`, updatedTask);
        console.log(`Updated task ${task.id} status to 'doing'.`);

        // 2. Spawn agent session
        const taskContent = `MoziBoard Task ${task.id}: ${task.title}\n${task.description}`;

        const result = await runCommand('openclaw', ['agent', '--agent', agentName, '--message', taskContent]);
        console.log(`Spawned session for agent ${agentName} with task: "MoziBoard Task ${task.id}: ${task.title}..."`);
        if (result) {
            console.log(`Spawn result: ${result}`);
        }

    } catch (error) {
        console.error(`Error processing task ${task.id}:`, error.message);
    }
}

async function pollTasks() {
    try {
        console.log('Polling for tasks...');

        // 1. Fetch all boards
        const boardsResponse = await axios.get(`${BASE_URL}/boards`);
        const boards = boardsResponse.data;

        if (!Array.isArray(boards)) {
            console.error('API response for boards is not an array.');
            return;
        }

        let allTasks = [];

        // 2. Fetch tasks for each board
        for (const board of boards) {
            try {
                const tasksResponse = await axios.get(`${BASE_URL}/boards/${board.id}/tasks`);
                const tasks = tasksResponse.data;

                if (Array.isArray(tasks)) {
                    // Ensure board_id is attached to each task
                    const tasksWithBoardId = tasks.map(t => ({
                        ...t,
                        board_id: t.board_id || board.id
                    }));
                    allTasks = allTasks.concat(tasksWithBoardId);
                }
            } catch (boardError) {
                console.error(`Error fetching tasks for board ${board.id}:`, boardError.message);
            }
        }

        // Filter tasks: list_id == 'todo' AND assignee_id in AGENTS
        const todoTasks = allTasks.filter(t => {
            if (!t.assignee_id) return false;

            // Check list_id instead of list/status
            const listStatus = (t.list_id || '').toLowerCase();
            const isTodo = listStatus === 'todo';

            const assignedLower = t.assignee_id.toLowerCase();
            const isAssigned = AGENTS.includes(assignedLower);

            return isTodo && isAssigned;
        });

        if (todoTasks.length > 0) {
            console.log(`Found ${todoTasks.length} tasks to process.`);
            for (const task of todoTasks) {
                await processTask(task);
            }
        } else {
            console.log('No matching tasks found.');
        }

    } catch (error) {
        // Handle API down or network errors gracefully
        console.error('Error polling tasks:', error.message);
    }
}

// Start polling
console.log('Starting dispatcher service...');
pollTasks(); // Run immediately on start
setInterval(pollTasks, POLL_INTERVAL);
