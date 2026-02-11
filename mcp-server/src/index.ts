import express from "express";
import cors from "cors";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { SSEServerTransport } from "@modelcontextprotocol/sdk/server/sse.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import axios from "axios";
import { z } from "zod";

// --- Configuration ---
const PORT = 3005;
const API_URL = process.env.API_URL || "http://localhost:8080/api";
const MCP_API_KEY = process.env.MCP_API_KEY;

if (!MCP_API_KEY) {
  console.warn("WARNING: MCP_API_KEY environment variable is not set. Authentication will fail.");
}

// --- Schemas ---
const listTasksSchema = z.object({
  board_id: z.string().optional().describe("Board ID to list tasks from (defaults to the first board)"),
});

const createTaskSchema = z.object({
  title: z.string().describe("Task title"),
  description: z.string().optional().describe("Task description"),
  list_id: z.string().optional().default("todo").describe("List/Status (todo, doing, done)"),
  assignee_id: z.string().optional().describe("Assignee user ID"),
});

const updateTaskSchema = z.object({
  id: z.number().describe("Task ID"),
  title: z.string().optional(),
  description: z.string().optional(),
  list_id: z.string().optional(),
  position: z.number().optional(),
  assignee_id: z.string().optional(),
  updated_by: z.string().optional().describe("Agent/User ID performing the update"),
});

// --- Server Factory ---
// We create a new MCP Server instance for each client connection
function createMcpServer() {
  const server = new Server(
    {
      name: "moziboard-mcp",
      version: "1.0.0",
    },
    {
      capabilities: {
        tools: {},
      },
    }
  );

  server.setRequestHandler(ListToolsRequestSchema, async () => {
    return {
      tools: [
        {
          name: "list_tasks",
          description: "List all tasks from a board",
          inputSchema: {
            type: "object",
            properties: {
              board_id: {
                type: "string",
                description: "Board ID to list tasks from (defaults to the first board)",
              },
            },
          },
        },
        {
          name: "create_task",
          description: "Create a new task",
          inputSchema: {
            type: "object",
            properties: {
              title: { type: "string", description: "Task title" },
              description: { type: "string", description: "Task description" },
              list_id: { type: "string", description: "List/Status (todo, doing, done)", default: "todo" },
              assignee_id: { type: "string", description: "Assignee user ID" },
            },
            required: ["title"],
          },
        },
        {
          name: "update_task",
          description: "Update an existing task",
          inputSchema: {
            type: "object",
            properties: {
              id: { type: "number", description: "Task ID" },
              title: { type: "string" },
              description: { type: "string" },
              list_id: { type: "string" },
              position: { type: "number" },
              assignee_id: { type: "string" },
              updated_by: { type: "string", description: "Agent/User ID performing the update" },
            },
            required: ["id"],
          },
        },
      ],
    };
  });

  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    const { name, arguments: args } = request.params;

    try {
      if (name === "list_tasks") {
        const { board_id } = listTasksSchema.parse(args || {});
        let targetBoardId = board_id;
        
        if (!targetBoardId) {
          const boards = await axios.get(`${API_URL}/boards`);
          if (boards.data.length > 0) {
            targetBoardId = boards.data[0].id;
          } else {
            return { content: [{ type: "text", text: "No boards found." }] };
          }
        }

        const tasks = await axios.get(`${API_URL}/boards/${targetBoardId}/tasks`);
        return {
          content: [{ type: "text", text: JSON.stringify(tasks.data, null, 2) }],
        };
      }

      if (name === "create_task") {
        const { title, description, list_id, assignee_id } = createTaskSchema.parse(args);
        
        const boards = await axios.get(`${API_URL}/boards`);
        if (boards.data.length === 0) {
          throw new Error("No boards found to create task in.");
        }
        const board_id = boards.data[0].id;

        const payload = {
          board_id,
          title,
          description,
          list_id,
          assignee_id,
        };

        const response = await axios.post(`${API_URL}/tasks`, payload);
        return {
          content: [{ type: "text", text: JSON.stringify(response.data, null, 2) }],
        };
      }

      if (name === "update_task") {
        const { id, ...updateData } = updateTaskSchema.parse(args);
        const response = await axios.put(`${API_URL}/tasks/${id}`, updateData);
        return {
          content: [{ type: "text", text: JSON.stringify(response.data, null, 2) }],
        };
      }

      throw new Error(`Unknown tool: ${name}`);
    } catch (error: any) {
      const errorMessage = error.response?.data 
        ? JSON.stringify(error.response.data) 
        : error.message;
        
      return {
        content: [{ type: "text", text: `Error: ${errorMessage}` }],
        isError: true,
      };
    }
  });

  return server;
}

// --- Express App ---
const app = express();
app.use(cors());
app.use(express.json()); // Restored for debugging, may conflict with streamable? 
// Wait, if I use streamableTransport.handleRequest(req, res), and req is already consumed by express.json(), it fails.
// But the user asked to "Restore 'app.use(express.json());'".
// Let's do it, but check if handleRequest supports pre-parsed body.
// The SDK docs say handleRequest(req, res, parsedBody?).
// So I should pass req.body if I use express.json().

// Auth Middleware
app.use((req, res, next) => {
  const authHeader = req.headers.authorization;
  if (!MCP_API_KEY) {
    // If no key configured, warn but maybe allow (dev mode)? 
    // Spec says "Require Authorization", so we should block.
    console.error("Auth blocked: MCP_API_KEY not set on server");
    res.status(500).json({ error: "Server configuration error: Auth key not set" });
    return;
  }

  if (!authHeader || authHeader !== `Bearer ${MCP_API_KEY}`) {
    res.status(401).json({ error: "Unauthorized" });
    return;
  }
  next();
});

// Store active transports
const transports = new Map<string, SSEServerTransport>();

// --- Streamable HTTP Transport (New Spec) ---
// One transport/server instance per app lifecycle, shared for all connections.
// State is managed internally by the SDK.
const streamableTransport = new StreamableHTTPServerTransport();
const streamableServer = createMcpServer();
streamableServer.connect(streamableTransport); // Connect once

app.post("/mcp", async (req, res) => {
  console.log("POST /mcp headers:", req.headers); // Debug headers
  console.log("POST /mcp body:", JSON.stringify(req.body).substring(0, 200)); // Debug body
  try {
    // Pass the request to the transport
    // The transport handles session IDs, message parsing, and response formatting
    // Since we use express.json(), we MUST pass req.body as 3rd arg.
    await streamableTransport.handleRequest(req, res, req.body);
  } catch (error: any) {
    console.error('Error handling /mcp request:', error);
    res.status(500).json({ error: error.message, stack: error.stack });
  }
});

app.get("/sse", async (req, res) => {
  console.log("New SSE connection");
  const transport = new SSEServerTransport("/messages", res);
  const server = createMcpServer();

  transports.set(transport.sessionId, transport);

  transport.onclose = () => {
    console.log("SSE connection closed", transport.sessionId);
    transports.delete(transport.sessionId);
  };

  await server.connect(transport);
});

app.post("/messages", async (req, res) => {
  const sessionId = req.query.sessionId as string;
  if (!sessionId) {
    res.status(400).send("Missing sessionId query parameter");
    return;
  }

  const transport = transports.get(sessionId);
  if (!transport) {
    res.status(404).send("Session not found");
    return;
  }

  // Pass the JSON-RPC message to the transport
  await transport.handlePostMessage(req, res);
});

app.listen(PORT, () => {
  console.log(`MoziBoard MCP Server (SSE) running on port ${PORT}`);
});
