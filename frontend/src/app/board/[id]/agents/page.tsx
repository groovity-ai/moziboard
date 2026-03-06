'use client';

import { useParams } from 'next/navigation';
import { Bot, Users } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';

export default function AgentsPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);

  return (
    <div className="flex flex-col h-full w-full">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="text-lg font-bold tracking-tight px-2 flex items-center gap-2">
            <Bot className="w-5 h-5 text-gray-500" />
            Active Agents
          </h1>
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

      <div className="flex-1 overflow-auto p-6">
        <div className="max-w-4xl mx-auto">
          <div className="rounded-xl border bg-card text-card-foreground shadow-sm">
            <div className="flex flex-col space-y-1.5 p-6">
              <h3 className="font-semibold leading-none tracking-tight">Board Agents</h3>
              <p className="text-sm text-muted-foreground">
                Manage the AI agents assigned to this board.
              </p>
            </div>
            <div className="p-6 pt-0">
               <div className="text-sm text-gray-400 italic">
                Agents assigned to this board will appear here. (Coming soon)
              </div>
            </div>
          </div>
        </div>
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </div>
  );
}