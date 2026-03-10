"use client";

import { createContext, useContext, useState, useCallback } from "react";

export type ChatTarget = {
  agentId: string;
  projectId?: string | null;
  displayName?: string | null;
  runtimeStatus?: string | null;
  subtitle?: string | null;
};

type ChatPanelContextValue = {
  isOpen: boolean;
  toggle: () => void;
  open: () => void;
  close: () => void;
  activeAgentId: string | null;
  activeTarget: ChatTarget | null;
  setActiveAgentId: (id: string | null) => void;
  setActiveTarget: (target: ChatTarget | null) => void;
};

const ChatPanelContext = createContext<ChatPanelContextValue | null>(null);

export const useChatPanel = () => {
  const context = useContext(ChatPanelContext);
  if (!context) {
    throw new Error("useChatPanel must be used within ChatPanelProvider");
  }
  return context;
};

export const ChatPanelProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [activeTarget, setActiveTarget] = useState<ChatTarget | null>(null);

  const toggle = useCallback(() => setIsOpen((prev) => !prev), []);
  const open = useCallback(() => setIsOpen(true), []);
  const close = useCallback(() => setIsOpen(false), []);
  const setActiveAgentId = useCallback((id: string | null) => {
    setActiveTarget(id ? { agentId: id, projectId: id, displayName: id } : null);
  }, []);

  return (
    <ChatPanelContext.Provider
      value={{
        isOpen,
        toggle,
        open,
        close,
        activeAgentId: activeTarget?.agentId || null,
        activeTarget,
        setActiveAgentId,
        setActiveTarget,
      }}
    >
      {children}
    </ChatPanelContext.Provider>
  );
};
