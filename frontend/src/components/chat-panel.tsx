"use client";

import { cn } from "@/lib/utils";
import { Chat } from "@/components/chat";
import { useChatPanel } from "@/providers/chat-panel-provider";

export const ChatPanel = () => {
    const { isOpen, close, activeAgentId } = useChatPanel();

    return (
        <div
            className={cn(
                "bg-sidebar hidden h-full shrink-0 overflow-hidden py-2 transition-all duration-200 ease-out md:block",
                isOpen ? "ml-1 w-96 pr-2 opacity-100" : "w-0 opacity-0",
            )}
        >
            {isOpen && (
                <div className="bg-background flex h-full flex-col overflow-hidden rounded-xl border">
                    <Chat
                        sessionKey={`mozi:global:${activeAgentId}`} // Unique session per agent
                        projectId={activeAgentId}
                        mode="panel"
                        onClose={close}
                        className="h-full border-l-0"
                    />
                </div>
            )}
        </div>
    );
};
