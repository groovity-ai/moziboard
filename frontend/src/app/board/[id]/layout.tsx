"use client";

import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { DashboardSidebar } from "@/components/DashboardSidebar";
import { ChatPanelProvider } from "@/providers/chat-panel-provider";
import { ChatPanel } from "@/components/chat-panel";

export default function BoardLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <ChatPanelProvider>
            <SidebarProvider
                className="bg-sidebar h-svh overflow-hidden"
                style={{ "--sidebar-width-icon": "3rem" } as React.CSSProperties}
            >
                <DashboardSidebar />
                <SidebarInset className="bg-background overflow-hidden rounded-none border md:rounded-xl md:peer-data-[variant=inset]:shadow-none md:peer-data-[variant=inset]:peer-data-[state=collapsed]:ml-1">
                    <main className="flex min-h-0 flex-1 flex-col overflow-hidden h-full">
                        {children}
                    </main>
                </SidebarInset>
                <ChatPanel />
            </SidebarProvider>
        </ChatPanelProvider>
    );
}
