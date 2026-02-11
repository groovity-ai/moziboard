'use client';

import { useParams } from 'next/navigation';
import { Board } from '@/components/Board';
import { KnowledgeBase } from '@/components/KnowledgeBase';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { Users, Layout, FileText } from 'lucide-react';

type TabType = 'kanban' | 'docs';

export default function BoardPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);
  const [activeTab, setActiveTab] = useState<TabType>('kanban');

  return (
    <main className="flex h-screen w-screen flex-col bg-gray-50 text-gray-900 dark:bg-zinc-950 dark:text-gray-100">
      <div className="flex items-center justify-between border-b bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex items-center gap-4">
          <h1 className="text-xl font-bold tracking-tight">
            <a href="/" className="hover:text-rose-500">Moziboard</a>
            <span className="mx-2 text-gray-400">/</span>
            Project
          </h1>

          {/* Tab Navigation */}
          <div className="flex items-center gap-1 rounded-lg bg-gray-100 p-1 dark:bg-zinc-800">
            <button
              onClick={() => setActiveTab('kanban')}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${activeTab === 'kanban'
                  ? 'bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-white'
                  : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
            >
              <Layout size={14} />
              Kanban
            </button>
            <button
              onClick={() => setActiveTab('docs')}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${activeTab === 'docs'
                  ? 'bg-white text-gray-900 shadow-sm dark:bg-zinc-700 dark:text-white'
                  : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
            >
              <FileText size={14} />
              Knowledge Base
            </button>
          </div>
        </div>

        <div className="flex gap-2">
          <button
            onClick={() => setShowMembers(true)}
            className="flex items-center gap-2 rounded-lg bg-gray-100 px-3 py-2 text-sm font-medium hover:bg-gray-200 dark:bg-zinc-800 dark:hover:bg-zinc-700"
          >
            <Users size={16} /> Members
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        {activeTab === 'kanban' ? (
          <Board boardId={id} />
        ) : (
          <KnowledgeBase boardId={id} />
        )}
      </div>

      <MemberManager
        boardId={id}
        isOpen={showMembers}
        onClose={() => setShowMembers(false)}
      />
    </main>
  );
}
