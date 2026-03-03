"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import type { ChatAttachment } from "@/components/chat/types";

export type Message = {
    id: string;
    role: "user" | "assistant" | "system";
    content: string;
    createdAt?: Date;
    runId?: string;
};

export type UseChatOptions = {
    sessionKey: string;
    projectId?: string | null;
    onError?: (error: Error) => void;
    onFinish?: () => void;
};

export type UseChatReturn = {
    messages: Message[];
    input: string;
    setInput: (input: string) => void;
    status: "idle" | "loading" | "streaming" | "error";
    error: Error | null;
    sendMessage: (text: string, attachments?: ChatAttachment[]) => Promise<void>;
    loadHistory: () => Promise<void>;
    abort: () => void;
    isLoading: boolean;
    isStreaming: boolean;
};

const generateId = () => `msg_${Date.now()}_${Math.random().toString(36).slice(2)}`;

export const useChat = ({
    sessionKey,
    projectId,
    onError,
    onFinish,
}: UseChatOptions): UseChatReturn => {
    const [messages, setMessages] = useState<Message[]>([]);
    const [input, setInput] = useState("");
    const [status, setStatus] = useState<"idle" | "loading" | "streaming" | "error">("idle");
    const [error, setError] = useState<Error | null>(null);

    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    const retryCountRef = useRef(0);
    const isConnectedRef = useRef(false);

    const connect = useCallback(() => {
        if (wsRef.current?.readyState === WebSocket.OPEN) return;
        if (!projectId) return; // Don't connect if no agent is selected

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;

        // In dev, assuming proxy routes /api/projects...
        // Adjust if MoziBoard has a different route, but we'll try to use the same existing Go backend
        const wsUrl = `${protocol}//localhost:4001/api/projects/${projectId}/ws?token=${projectId}&client=webchat`;

        console.log(`[use-chat WS] Connecting to ${wsUrl}...`);
        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
            console.log('[use-chat WS] Connected');
            isConnectedRef.current = true;
            retryCountRef.current = 0;
        };

        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                handleWsMessage(data);
            } catch (e) {
                console.error('[use-chat WS] Parse error:', e);
            }
        };

        ws.onclose = (event) => {
            console.log('[use-chat WS] Closed', event.code, event.reason);
            isConnectedRef.current = false;
            wsRef.current = null;

            // Reconnect logic
            const delay = Math.min(3000 * Math.pow(2, retryCountRef.current), 30000);
            retryCountRef.current++;
            reconnectTimeoutRef.current = setTimeout(connect, delay);
        };

        ws.onerror = (err) => {
            console.error('[use-chat WS] Error:', err);
            ws.close();
        };

        wsRef.current = ws;
    }, [sessionKey]);

    useEffect(() => {
        // Reset state when project changes
        setMessages([]);
        setStatus("idle");
        setError(null);

        if (wsRef.current) {
            wsRef.current.close();
        }

        if (projectId) {
            connect();
        }

        return () => {
            wsRef.current?.close();
            if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
        };
    }, [connect, projectId]);


    const handleWsMessage = (data: any) => {
        // 1. Handshake Complete -> Fetch History
        if (data.type === 'res' && data.id === 'init-1' && data.ok) {
            console.log('[use-chat WS] Handshake Complete. Fetching History...');
            wsRef.current?.send(JSON.stringify({
                type: "req",
                id: "fetch-history",
                method: "chat.history",
                params: { sessionKey, limit: 20 }
            }));
            return;
        }

        // 2. Handle History
        if (data.type === 'res' && data.id === 'fetch-history' && data.ok) {
            const historyItems = data.payload?.messages || [];
            const formattedHistory: Message[] = historyItems.map((item: any) => {
                let text = '';
                const contentObj = item.content || item.message?.content || item.message;
                if (typeof contentObj === 'string') {
                    text = contentObj;
                } else if (Array.isArray(contentObj)) {
                    text = contentObj.filter((p: any) => p.type === 'text').map((p: any) => p.text).join('');
                }
                // Clean tags
                if (item.role === 'user') {
                    text = text.replace(/Conversation info.*?```\n\n/g, '').replace(/^\[.*? UTC\]\s*/g, '');
                } else if (item.role === 'assistant') {
                    text = text.replace(/\[\[reply_to_current\]\]/g, '');
                }
                return {
                    id: item.id || generateId(),
                    role: item.role === 'user' ? 'user' : 'assistant',
                    content: text.trim()
                };
            }).filter((m: Message) => m.content.trim() !== '');

            setMessages(formattedHistory);
            setStatus("idle");
            return;
        }

        // 3. Handle Streamed Chat Event
        if (data.type === 'event' && data.event === 'chat') {
            if (data.payload?.state === 'error' && data.payload?.errorMessage) {
                setMessages(prev => [...prev, { id: generateId(), role: 'system', content: `Error: ${data.payload.errorMessage}` }]);
                setStatus("error");
                return;
            }

            let contentStr = '';
            if (typeof data.payload?.message === 'string') {
                contentStr = data.payload.message;
            } else if (data.payload?.message?.content) {
                if (typeof data.payload.message.content === 'string') {
                    contentStr = data.payload.message.content;
                } else if (Array.isArray(data.payload.message.content)) {
                    contentStr = data.payload.message.content.filter((p: any) => p.type === 'text').map((p: any) => p.text).join('');
                }
            } else if (typeof data.payload?.text === 'string') {
                contentStr = data.payload.text;
            }

            contentStr = contentStr.replace(/\[\[reply_to_current\]\]/g, '').trim();
            if (!contentStr) return;

            setMessages(prev => {
                const runId = data.payload?.runId;
                if (runId) {
                    const existingIdx = prev.findIndex(m => m.runId === runId);
                    if (existingIdx >= 0) {
                        const newMessages = [...prev];
                        newMessages[existingIdx] = { ...newMessages[existingIdx], content: contentStr };
                        return newMessages;
                    }
                }
                return [...prev, { id: generateId(), runId, role: 'assistant', content: contentStr }];
            });

            if (data.payload?.state === "finished") {
                setStatus("idle");
                onFinish?.();
            } else {
                setStatus("streaming");
            }
        }

        // 4. Handle System / Gateway Errors
        if (data.type === 'res' && !data.ok && data.error) {
            setMessages(prev => [...prev, { id: generateId(), role: 'system', content: `Gateway Error: ${data.error.message || 'Rejected'}` }]);
            setStatus("error");
        }
    };

    const sendMessage = useCallback(
        async (text: string, attachments?: ChatAttachment[]) => {
            const trimmed = text.trim();
            if (!trimmed || !isConnectedRef.current || !wsRef.current) return;

            const userMessage: Message = { id: generateId(), role: "user", content: trimmed, createdAt: new Date() };
            setMessages((prev) => [...prev, userMessage]);
            setInput("");
            setStatus("streaming");
            setError(null);

            const frameId = generateId();
            const payload = {
                type: "req",
                id: frameId,
                method: "chat.send",
                params: { message: trimmed, sessionKey, idempotencyKey: frameId }
            };

            try {
                wsRef.current.send(JSON.stringify(payload));
            } catch (err) {
                setStatus("error");
                setError(new Error("Failed to send WebSocket message"));
            }
        },
        [sessionKey]
    );

    const loadHistory = useCallback(async () => {
        setStatus("loading");
        if (isConnectedRef.current && wsRef.current) {
            wsRef.current.send(JSON.stringify({
                type: "req",
                id: "fetch-history",
                method: "chat.history",
                params: { sessionKey, limit: 20 }
            }));
        } else {
            setStatus("idle"); // Will fetch when connected
        }
    }, [sessionKey]);

    const abort = useCallback(() => {
        // There isn't a direct abort on the OpenClaw WS protocol defined in the snippet,
        // but we can set to idle
        setStatus("idle");
    }, []);

    return {
        messages,
        input,
        setInput,
        status,
        error,
        sendMessage,
        loadHistory,
        abort,
        isLoading: status === "loading",
        isStreaming: status === "streaming",
    };
};
