import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
console.log("Server imported:", !!Server);
console.log("Stdio imported:", !!StdioServerTransport);
