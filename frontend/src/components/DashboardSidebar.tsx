"use client";

import * as React from "react";
import { useParams, usePathname, useRouter } from "next/navigation";
import { LayoutDashboard, SquareKanban, FileText, Settings, Bot, MessageSquare, ActivitySquare } from "lucide-react";
import { useChatPanel } from "@/providers/chat-panel-provider";
import { toast } from "sonner";

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

export function DashboardSidebar({
    ...props
}: React.ComponentProps<typeof Sidebar>) {
    const params = useParams();
    const id = params.id as string;
    const pathname = usePathname();
    const { toggle, isOpen, activeAgentId, setActiveAgentId, open: openChat } = useChatPanel();

    const [agents, setAgents] = React.useState<any[]>([]);
    const [isLoading, setIsLoading] = React.useState(true);

    React.useEffect(() => {
        const fetchAgents = async () => {
            try {
                const res = await fetch('/api/agents/sync/aiagenz');
                if (res.ok) {
                    const data = await res.json();
                    setAgents(Array.isArray(data) ? data : data.data || []);
                }
            } catch (error) {
                console.error("Failed to fetch AiAgenz agents", error);
            } finally {
                setIsLoading(false);
            }
        };

        fetchAgents();
    }, []);

    return (
        <Sidebar collapsible="icon" variant="inset" {...props}>
            <SidebarHeader className="h-12 flex items-center px-4 font-bold tracking-tight mt-2 text-rose-500">
                <LayoutDashboard className="mr-2 h-5 w-5" />
                <span className="group-data-[collapsible=icon]:hidden">MoziBoard</span>
            </SidebarHeader>
            <SidebarContent className="overflow-hidden mt-4">
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
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    tooltip="Agent Chat"
                                    isActive={isOpen}
                                    onClick={(e) => {
                                        e.preventDefault();
                                        toggle();
                                    }}
                                >
                                    <MessageSquare />
                                    <span>Agent Chat</span>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>

                <SidebarGroup>
                    <SidebarGroupLabel>AiAgenz Workforce</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            {isLoading ? (
                                <SidebarMenuItem>
                                    <SidebarMenuButton disabled>
                                        <Bot className="animate-bounce" />
                                        <span>Syncing agents...</span>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ) : agents.length === 0 ? (
                                <SidebarMenuItem>
                                    <SidebarMenuButton disabled>
                                        <Bot className="opacity-50" />
                                        <span className="opacity-50">No agents found</span>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ) : (
                                agents.map((agent) => (
                                    <SidebarMenuItem key={agent.id}>
                                        <SidebarMenuButton
                                            tooltip={`Chat with ${agent.name}`}
                                            isActive={isOpen && activeAgentId === agent.id}
                                            onClick={(e) => {
                                                e.preventDefault();
                                                setActiveAgentId(agent.id);
                                                openChat();
                                                toast.success(`Connected to ${agent.name}`);
                                            }}
                                            className={activeAgentId === agent.id ? "text-primary" : ""}
                                        >
                                            <div className="flex items-center gap-2 w-full">
                                                <div className="relative">
                                                    <img src={`https://api.dicebear.com/7.x/bottts/svg?seed=${agent.id}`} alt="Bot avatar" className="w-5 h-5 rounded bg-muted" />
                                                    {agent.status === "running" && (
                                                        <span className="absolute -bottom-0.5 -right-0.5 w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                                                    )}
                                                </div>
                                                <span className="truncate flex-1">{agent.name}</span>
                                            </div>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                ))
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
