"use client";

import { Bot, Loader2, Wifi, WifiOff, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export type ChatHeaderProps = {
  mode?: "panel" | "full";
  onClose?: () => void;
  isStreaming?: boolean;
  className?: string;
  title?: string;
  subtitle?: string;
  runtimeStatus?: string;
  isConnected?: boolean;
  connectionLabel?: string;
};

function runtimeTone(status?: string) {
  const value = (status || "unknown").toLowerCase();
  if (value === "running" || value === "online") return "bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-300";
  if (value === "stopped" || value === "offline") return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
  if (value === "exited" || value === "failed" || value === "error") return "bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-300";
  return "bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300";
}

export const ChatHeader = ({
  mode = "full",
  onClose,
  isStreaming,
  className,
  title,
  subtitle,
  runtimeStatus,
  isConnected,
  connectionLabel,
}: ChatHeaderProps) => {
  return (
    <div
      className={cn(
        "flex items-center justify-between border-b px-4 py-3",
        mode === "panel" && "px-3 py-3",
        className,
      )}
    >
      <div className="min-w-0 flex items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-rose-100 text-rose-600 dark:bg-rose-900/20 dark:text-rose-300">
          <Bot className="h-4 w-4" />
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h2
              className={cn(
                "truncate text-foreground font-semibold",
                mode === "panel" ? "text-sm" : "text-lg",
              )}
            >
              {title || "Agent Chat"}
            </h2>
            {runtimeStatus && (
              <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide", runtimeTone(runtimeStatus))}>
                {runtimeStatus}
              </span>
            )}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {subtitle && <span className="truncate">{subtitle}</span>}
            <span className="flex items-center gap-1">
              {isConnected ? <Wifi className="h-3 w-3 text-green-500" /> : <WifiOff className="h-3 w-3 text-zinc-400" />}
              {connectionLabel || (isConnected ? "Connected" : "Disconnected")}
            </span>
            {isStreaming && (
              <span className="flex items-center gap-1 text-rose-600 dark:text-rose-300">
                <Loader2 className="h-3 w-3 animate-spin" />
                Generating...
              </span>
            )}
          </div>
        </div>
      </div>

      {mode === "panel" && onClose && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              onClick={onClose}
              className="h-8 w-8"
            >
              <X className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Close</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
};
