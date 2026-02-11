import express from "express";
import cors from "cors";
import crypto from "crypto";
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
app.use(express.json());

// Auth Middleware
app.use((req, res, next) => {
  const authHeader = req.headers.authorization;
  if (!MCP_API_KEY) {
    console.error("Auth blocked: MCP_API_KEY not set on server");
    res.status(500).json({ error: "Server configuration error: Auth key not set" });
    return;
  }

  const expected = `Bearer ${MCP_API_KEY}`;
  if (!authHeader || authHeader.length !== expected.length ||
    !crypto.timingSafeEqual(Buffer.from(authHeader), Buffer.from(expected))) {
    res.status(401).json({ error: "Unauthorized" });
    return;
  }
  next();
});

// --- Transport Storage ---
const sseTransports = new Map<string, SSEServerTransport>();
const streamableTransports = new Map<string, StreamableHTTPServerTransport>();

// --- Streamable HTTP Transport (MCP Spec 2025-03-26) ---
// Each client session gets its own transport + server instance.

app.post("/mcp", async (req, res) => {
  const sessionId = req.headers["mcp-session-id"] as string | undefined;

  console.log(`[MCP] POST /mcp method=${req.body?.method} sessionId=${sessionId || "none"}`);

  try {
    if (sessionId && streamableTransports.has(sessionId)) {
      // Existing session — route request to its transport
      const transport = streamableTransports.get(sessionId)!;
      await transport.handleRequest(req, res, req.body);
    } else if (!sessionId && req.body?.method === "initialize") {
      // New session — create transport + server, then handle the initialize request
      const transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: () => crypto.randomUUID(),
      });

      transport.onclose = () => {
        const sid = transport.sessionId;
        if (sid) {
          console.log(`[MCP] Session closed: ${sid}`);
          streamableTransports.delete(sid);
        }
      };

      const server = createMcpServer();
      await server.connect(transport);

      // handleRequest processes the initialize message and assigns sessionId
      await transport.handleRequest(req, res, req.body);

      // Register the session AFTER handleRequest so sessionId is populated
      if (transport.sessionId) {
        streamableTransports.set(transport.sessionId, transport);
        console.log(`[MCP] New session created: ${transport.sessionId}`);
      }
    } else {
      // Invalid request — no session and not an initialize
      res.status(400).json({
        jsonrpc: "2.0",
        error: {
          code: -32000,
          message: "Bad Request: No valid session. Send an initialize request first.",
        },
        id: req.body?.id ?? null,
      });
    }
  } catch (error: any) {
    console.error("[MCP] Error handling POST /mcp:", error);
    if (!res.headersSent) {
      res.status(500).json({
        jsonrpc: "2.0",
        error: { code: -32603, message: error.message },
        id: req.body?.id ?? null,
      });
    }
  }
});

// GET /mcp — SSE stream for server-initiated messages within a session
app.get("/mcp", async (req, res) => {
  const sessionId = req.headers["mcp-session-id"] as string | undefined;
  console.log(`[MCP] GET /mcp sessionId=${sessionId || "none"}`);

  if (sessionId && streamableTransports.has(sessionId)) {
    const transport = streamableTransports.get(sessionId)!;
    await transport.handleRequest(req, res);
  } else {
    res.status(400).json({
      jsonrpc: "2.0",
      error: { code: -32000, message: "No valid session for GET request" },
      id: null,
    });
  }
});

// DELETE /mcp — Session cleanup
app.delete("/mcp", async (req, res) => {
  const sessionId = req.headers["mcp-session-id"] as string | undefined;
  console.log(`[MCP] DELETE /mcp sessionId=${sessionId || "none"}`);

  if (sessionId && streamableTransports.has(sessionId)) {
    const transport = streamableTransports.get(sessionId)!;
    await transport.handleRequest(req, res);
  } else {
    res.status(400).json({
      jsonrpc: "2.0",
      error: { code: -32000, message: "No valid session to delete" },
      id: null,
    });
  }
});

// --- Legacy SSE Transport ---
app.get("/sse", async (req, res) => {
  console.log("[SSE] New SSE connection");
  const transport = new SSEServerTransport("/messages", res);
  const server = createMcpServer();

  sseTransports.set(transport.sessionId, transport);

  transport.onclose = () => {
    console.log("[SSE] Connection closed:", transport.sessionId);
    sseTransports.delete(transport.sessionId);
  };

  await server.connect(transport);
});

app.post("/messages", async (req, res) => {
  const sessionId = req.query.sessionId as string;
  if (!sessionId) {
    res.status(400).send("Missing sessionId query parameter");
    return;
  }

  const transport = sseTransports.get(sessionId);
  if (!transport) {
    res.status(404).send("Session not found");
    return;
  }

  await transport.handlePostMessage(req, res);
});

// --- Start Server ---
app.listen(PORT, () => {
  console.log(`MoziBoard MCP Server running on port ${PORT}`);
  console.log(`  Streamable HTTP: POST/GET/DELETE /mcp`);
  console.log(`  Legacy SSE:      GET /sse + POST /messages`);
});
