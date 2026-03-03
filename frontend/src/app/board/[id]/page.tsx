'use client';

import { useParams } from 'next/navigation';
import { Board } from '@/components/Board';
import { KnowledgeBase } from '@/components/KnowledgeBase';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { Users, Layout, FileText, Bot } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";

type TabType = 'kanban' | 'docs';

export default function BoardPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);
  const [activeTab, setActiveTab] = useState<TabType>('kanban');

  return (
    <div className="flex flex-col h-full w-full">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="text-lg font-bold tracking-tight px-2">
            Sprint Workspace
          </h1>
          {/* Tab Navigation */}
          <div className="flex items-center gap-1 rounded-lg bg-gray-100 p-1 dark:bg-zinc-800 ml-4">
            <button
              onClick={() => setActiveTab('kanban')}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1 text-sm font-medium transition-colors ${activeTab === 'kanban'
                ? 'bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-white'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
            >
              <Layout size={14} />
              Kanban
            </button>
            <button
              onClick={() => setActiveTab('docs')}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1 text-sm font-medium transition-colors ${activeTab === 'docs'
                ? 'bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-white'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
            >
              <FileText size={14} />
              Docs
            </button>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowMembers(true)}
            className="flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium hover:bg-gray-200 dark:bg-zinc-800 dark:hover:bg-zinc-700"
          >
            <Users size={16} /> Members
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-hidden">
        <ResizablePanelGroup orientation="horizontal" className="h-full">
          {/* Agents / Quick Filters Panel */}
          <ResizablePanel
            defaultSize={15}
            minSize={10}
            maxSize={20}
            collapsible
            collapsedSize={4}
            className="hidden md:block bg-zinc-50/50 dark:bg-zinc-900/20"
          >
            <div className="p-4 flex flex-col h-full">
              <h3 className="font-semibold text-sm text-gray-500 flex items-center gap-2 mb-4">
                <Bot size={16} /> Active Agents
              </h3>
              <div className="text-sm text-gray-400 italic">
                Agents assigned to this board will appear here.
              </div>
            </div>
          </ResizablePanel>

          <ResizableHandle className="hover:bg-rose-500/50 w-[2px] bg-border transition-colors focus-visible:ring-0 focus-visible:ring-offset-0 hidden md:flex" />

          {/* Main Content */}
          <ResizablePanel minSize={50}>
            <div className="h-full w-full overflow-hidden">
              {activeTab === 'kanban' ? (
                <Board boardId={id} />
              ) : (
                <KnowledgeBase boardId={id} />
              )}
            </div>
          </ResizablePanel>
        </ResizablePanelGroup>
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </div>
  );
}
