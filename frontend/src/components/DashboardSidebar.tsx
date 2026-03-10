"use client";

import * as React from "react";
import { useParams, usePathname } from "next/navigation";
import { LayoutDashboard, SquareKanban, FileText, Settings, Bot, ActivitySquare, PlugZap, ChevronRight } from "lucide-react";
import { useChatPanel } from "@/providers/chat-panel-provider";
import { toast } from "sonner";
import { buildAuthHeaders } from "@/lib/auth";

import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarHeader,
    SidebarRail,
    SidebarMenu,
    SidebarMenuItem,
    SidebarMenuButton,
    SidebarGroup,
    SidebarGroupLabel,
    SidebarGroupContent,
} from "@/components/ui/sidebar";

type BoardAgentEntry = {
    id: number;
    board_id: string;
    agent_id: string;
    active: boolean;
    agent?: {
        id: string;
        display_name?: string;
        is_native_clawn?: boolean;
        provider?: string;
        engine?: string;
        status?: string;
        avatar?: string;
    };
};

type ClawnProject = {
    project_id: string;
    display_name: string;
    status?: string;
    already_connected?: boolean;
};

function runtimeStatusTone(status?: string) {
    const value = (status || "unknown").toLowerCase();
    if (value === "running" || value === "online") return "bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-300";
    if (value === "stopped" || value === "offline") return "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";
    if (value === "exited" || value === "failed" || value === "error") return "bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-300";
    return "bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300";
}

function resolveChatProjectId(agentId: string) {
    if (agentId.startsWith("clawn-project:")) {
        return agentId.replace(/^clawn-project:/, "");
    }
    return agentId;
}

function runtimeDot(status?: string) {
    const value = (status || "unknown").toLowerCase();
    if (value === "running" || value === "online") return "bg-green-500";
    if (value === "failed" || value === "error" || value === "offline" || value === "stopped") return "bg-red-500";
    return "bg-amber-500";
}

export function DashboardSidebar({
    ...props
}: React.ComponentProps<typeof Sidebar>) {
    const params = useParams();
    const id = params.id as string;
    const pathname = usePathname();
    const { isOpen, activeAgentId, setActiveTarget, open: openChat } = useChatPanel();

    const [connectedAgents, setConnectedAgents] = React.useState<BoardAgentEntry[]>([]);
    const [availableCount, setAvailableCount] = React.useState(0);
    const [isLoading, setIsLoading] = React.useState(true);

    React.useEffect(() => {
        const run = async () => {
            try {
                setIsLoading(true);
                const [boardAgentsRes, clawnProjectsRes] = await Promise.all([
                    fetch(`/api/boards/${id}/agents`),
                    fetch(`/api/integrations/clawn/projects?board_id=${id}`, {
                        headers: buildAuthHeaders(),
                        credentials: 'include',
                    }),
                ]);

                if (boardAgentsRes.ok) {
                    const boardAgentsData = await boardAgentsRes.json();
                    const normalized = Array.isArray(boardAgentsData) ? boardAgentsData : [];
                    setConnectedAgents(normalized.filter((entry) => entry?.active !== false && entry?.agent));
                } else {
                    setConnectedAgents([]);
                }

                if (clawnProjectsRes.ok) {
                    const clawnProjectsData = await clawnProjectsRes.json();
                    const projects: ClawnProject[] = Array.isArray(clawnProjectsData) ? clawnProjectsData : [];
                    setAvailableCount(projects.filter((item) => !item.already_connected).length);
                } else {
                    setAvailableCount(0);
                }
            } catch (error) {
                console.error("Failed to fetch board workforce state", error);
                setConnectedAgents([]);
                setAvailableCount(0);
            } finally {
                setIsLoading(false);
            }
        };

        run();
    }, [id]);

    return (
        <Sidebar collapsible="icon" variant="inset" {...props}>
            <SidebarHeader className="mt-2 px-3">
                <div className="flex items-center gap-3 rounded-2xl border border-rose-500/10 bg-gradient-to-br from-rose-500/10 via-background to-background px-3 py-3 shadow-sm">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-rose-500 text-white shadow-sm">
                        <LayoutDashboard className="h-5 w-5" />
                    </div>
                    <div className="min-w-0 group-data-[collapsible=icon]:hidden">
                        <div className="flex items-center gap-2">
                            <span className="truncate font-bold tracking-tight text-rose-500">ClawnBoard</span>
                            <span className="rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-rose-700 dark:bg-rose-900/20 dark:text-rose-300">beta</span>
                        </div>
                        <div className="mt-0.5 text-[11px] text-muted-foreground">Mission control for boards, docs, and agent ops.</div>
                    </div>
                </div>
            </SidebarHeader>
            <SidebarContent className="mt-4 overflow-hidden px-2 pb-3">
                <SidebarGroup>
                    <SidebarGroupLabel>Project Workspace</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    tooltip="Kanban"
                                    isActive={pathname === `/board/${id}` || pathname === `/board/${id}/kanban`}
                                >
                                    <a href={`/board/${id}`}>
                                        <SquareKanban />
                                        <span>Kanban Board</span>
                                    </a>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    tooltip="Knowledge Base"
                                    isActive={pathname === `/board/${id}/docs`}
                                >
                                    <a href={`/board/${id}/docs`}>
                                        <FileText />
                                        <span>Knowledge Base</span>
                                    </a>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>

                <SidebarGroup>
                    <SidebarGroupLabel>
                        <div className="flex w-full items-center justify-between gap-2">
                            <span>Connected Workforce</span>
                            {!isLoading && availableCount > 0 && (
                                <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                                    +{availableCount} available
                                </span>
                            )}
                        </div>
                    </SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu className="max-h-[320px] overflow-y-auto pr-1">
                            {isLoading ? (
                                <SidebarMenuItem>
                                    <SidebarMenuButton disabled className="rounded-xl border border-dashed dark:border-zinc-800">
                                        <Bot className="animate-bounce text-rose-500" />
                                        <span>Loading workforce...</span>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ) : connectedAgents.length === 0 ? (
                                <SidebarMenuItem>
                                    <SidebarMenuButton disabled className="rounded-xl border border-dashed dark:border-zinc-800">
                                        <Bot className="opacity-50" />
                                        <span className="opacity-50">No connected agents</span>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ) : (
                                connectedAgents.map((entry) => {
                                    const agent = entry.agent;
                                    if (!agent) return null;
                                    return (
                                        <SidebarMenuItem key={agent.id}>
                                            <SidebarMenuButton
                                                tooltip={`Chat with ${agent.display_name || agent.id}`}
                                                isActive={isOpen && activeAgentId === agent.id}
                                                onClick={(e) => {
                                                    e.preventDefault();
                                                    setActiveTarget({
                                                        agentId: agent.id,
                                                        projectId: resolveChatProjectId(agent.id),
                                                        displayName: agent.display_name || agent.id,
                                                        runtimeStatus: agent.status || 'unknown',
                                                        subtitle: agent.provider === 'clawn' ? `Clawn • ${agent.engine || 'runtime'}` : (agent.engine || agent.provider || 'runtime'),
                                                    });
                                                    openChat();
                                                    toast.success(`Connected to ${agent.display_name || agent.id}`);
                                                }}
                                                className="rounded-xl px-2 py-2"
                                            >
                                                <div className="flex w-full min-w-0 items-center gap-2">
                                                    <div className="relative shrink-0">
                                                        <img src={`https://api.dicebear.com/7.x/bottts/svg?seed=${agent.id}`} alt="Bot avatar" className="h-7 w-7 rounded-md bg-muted" />
                                                        <span className={`absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full ${runtimeDot(agent.status)}`}></span>
                                                    </div>
                                                    <div className="min-w-0 flex-1">
                                                        <div className="truncate text-sm">{agent.display_name || agent.id}</div>
                                                        <div className="mt-1 flex items-center gap-2">
                                                            <span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${runtimeStatusTone(agent.status)}`}>
                                                                {agent.status || 'unknown'}
                                                            </span>
                                                        </div>
                                                    </div>
                                                    <ChevronRight className="h-3.5 w-3.5 shrink-0 opacity-40" />
                                                </div>
                                            </SidebarMenuButton>
                                        </SidebarMenuItem>
                                    );
                                })
                            )}

                            {!isLoading && (
                                <SidebarMenuItem>
                                    <a
                                        href={`/board/${id}/agents`}
                                        className="mt-2 flex items-center justify-between rounded-2xl border border-dashed px-3 py-3 text-sm font-medium text-muted-foreground transition hover:border-rose-500/30 hover:bg-rose-50/60 hover:text-foreground dark:border-zinc-800 dark:hover:bg-zinc-900"
                                    >
                                        <div className="flex items-center gap-2">
                                            <PlugZap className="h-4 w-4 text-rose-500" />
                                            <span>{availableCount > 0 ? `Connect ${availableCount} more from Clawn` : 'Manage workforce connections'}</span>
                                        </div>
                                        <ChevronRight className="h-4 w-4" />
                                    </a>
                                </SidebarMenuItem>
                            )}
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>

                <SidebarGroup>
                    <SidebarGroupLabel>Management</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    tooltip="Agents"
                                    isActive={pathname === `/board/${id}/agents`}
                                >
                                    <a href={`/board/${id}/agents`}>
                                        <Bot />
                                        <span>Agents</span>
                                    </a>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    tooltip="Ops"
                                    isActive={pathname === `/board/${id}/ops`}
                                >
                                    <a href={`/board/${id}/ops`}>
                                        <ActivitySquare />
                                        <span>Ops</span>
                                    </a>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    asChild
                                    tooltip="Settings"
                                >
                                    <a href={`/board/${id}/settings`}>
                                        <Settings />
                                        <span>Settings</span>
                                    </a>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>
            </SidebarContent>
            <SidebarFooter className="justify-center group-data-[collapsible=icon]:px-0">
                {/* Footer Content */}
            </SidebarFooter>
            <SidebarRail />
        </Sidebar>
    );
}
