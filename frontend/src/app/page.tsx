'use client';

import React, { useState } from 'react';
import useSWR, { mutate } from 'swr';
import Link from 'next/link';
import { Plus, Layout } from 'lucide-react';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export default function Dashboard() {
  const { data: boards } = useSWR('/api/boards', fetcher);
  const [isCreating, setIsCreating] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newDesc, setNewDesc] = useState('');

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    await fetch('/api/boards', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: newTitle, description: newDesc }),
    });
    mutate('/api/boards');
    setIsCreating(false);
    setNewTitle('');
    setNewDesc('');
  };

  return (
    <main className="flex min-h-screen w-full flex-col bg-gray-50 text-gray-900 dark:bg-zinc-950 dark:text-gray-100">
      <div className="flex items-center justify-between border-b bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-rose-500 text-white shadow-lg shadow-rose-500/20">
                <Layout size={20} />
            </div>
            <h1 className="text-2xl font-bold tracking-tight">Moziboard Dashboard</h1>
        </div>
        <button 
            onClick={() => setIsCreating(true)}
            className="flex items-center gap-2 rounded-lg bg-black px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-black"
        >
            <Plus size={16} /> New Project
        </button>
      </div>

      <div className="container mx-auto p-8">
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {boards?.map((board: any) => (
                <Link key={board.id} href={`/board/${board.id}`}>
                    <div className="group relative flex h-48 flex-col justify-between rounded-2xl border bg-white p-6 shadow-sm transition-all hover:-translate-y-1 hover:shadow-xl hover:shadow-rose-500/10 dark:border-zinc-800 dark:bg-zinc-900">
                        <div>
                            <h3 className="text-lg font-bold group-hover:text-rose-500">{board.title}</h3>
                            <p className="mt-2 text-sm text-gray-500 line-clamp-3">{board.description}</p>
                        </div>
                        <div className="text-xs font-medium text-gray-400">
                            ID: {board.id}
                        </div>
                    </div>
                </Link>
            ))}
            
            {/* Empty State / Add New Card */}
            <button 
                onClick={() => setIsCreating(true)}
                className="flex h-48 flex-col items-center justify-center rounded-2xl border-2 border-dashed border-gray-300 bg-transparent text-gray-400 hover:border-rose-500 hover:bg-rose-50 hover:text-rose-500 dark:border-zinc-700 dark:hover:bg-zinc-900"
            >
                <Plus size={32} />
                <span className="mt-2 text-sm font-medium">Create New Board</span>
            </button>
        </div>
      </div>

      {/* Create Modal */}
      {isCreating && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
            <form onSubmit={handleCreate} className="w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl dark:bg-zinc-900">
                <h2 className="mb-4 text-xl font-bold">Create New Project</h2>
                <div className="mb-4 space-y-3">
                    <div>
                        <label className="mb-1 block text-sm font-medium">Project Title</label>
                        <input 
                            required
                            type="text" 
                            value={newTitle}
                            onChange={(e) => setNewTitle(e.target.value)}
                            className="w-full rounded-lg border bg-gray-50 px-3 py-2 outline-none focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-800"
                            placeholder="e.g. Website Redesign"
                        />
                    </div>
                    <div>
                        <label className="mb-1 block text-sm font-medium">Description</label>
                        <textarea 
                            value={newDesc}
                            onChange={(e) => setNewDesc(e.target.value)}
                            className="w-full rounded-lg border bg-gray-50 px-3 py-2 outline-none focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-800"
                            placeholder="Short description..."
                        />
                    </div>
                </div>
                <div className="flex justify-end gap-2">
                    <button type="button" onClick={() => setIsCreating(false)} className="rounded-lg px-4 py-2 hover:bg-gray-100 dark:hover:bg-zinc-800">Cancel</button>
                    <button type="submit" className="rounded-lg bg-rose-500 px-4 py-2 text-white hover:bg-rose-600">Create Project</button>
                </div>
            </form>
        </div>
      )}
    </main>
  );
}
