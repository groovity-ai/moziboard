'use client';

import { useParams } from 'next/navigation';
import { KnowledgeBase } from '@/components/KnowledgeBase';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { Users, FileText } from 'lucide-react';
import { SidebarTrigger } from '@/components/ui/sidebar';

export default function DocsPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);

  return (
    <div className="flex flex-col h-full w-full">
      <header className="flex h-14 shrink-0 items-center justify-between gap-2 border-b px-4">
        <div className="flex items-center gap-2">
          <SidebarTrigger />
          <h1 className="text-lg font-bold tracking-tight px-2 flex items-center gap-2">
            <FileText className="w-5 h-5 text-gray-400" />
            Knowledge Base
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

      <div className="flex-1 overflow-hidden h-full">
         <KnowledgeBase boardId={id} />
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </div>
  );
}