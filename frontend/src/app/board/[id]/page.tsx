'use client';

import { useParams } from 'next/navigation';
import { Board } from '@/components/Board';
import { MemberManager } from '@/components/MemberManager';
import { useState } from 'react';
import { Users } from 'lucide-react';

export default function BoardPage() {
  const params = useParams();
  const id = params.id as string;
  const [showMembers, setShowMembers] = useState(false);

  return (
    <main className="flex h-screen w-screen flex-col bg-gray-50 text-gray-900 dark:bg-zinc-950 dark:text-gray-100">
      <div className="flex items-center justify-between border-b bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <h1 className="text-xl font-bold tracking-tight">
            <a href="/" className="hover:text-rose-500">Moziboard</a> 
            <span className="mx-2 text-gray-400">/</span>
            Project
        </h1>
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
        <Board boardId={id} />
      </div>

      <MemberManager 
        boardId={id} 
        isOpen={showMembers} 
        onClose={() => setShowMembers(false)} 
      />
    </main>
  );
}
