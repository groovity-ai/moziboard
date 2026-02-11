'use client';

import React, { useState } from 'react';
import useSWR, { mutate } from 'swr';
import { Plus, FileText, Trash2, Save, X, ChevronLeft } from 'lucide-react';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

type Doc = {
    id: number;
    board_id: string;
    title: string;
    content: string;
    created_at: string;
    updated_at: string;
};

interface KnowledgeBaseProps {
    boardId: string;
}

export function KnowledgeBase({ boardId }: KnowledgeBaseProps) {
    const { data: docs } = useSWR<Doc[]>(`/api/boards/${boardId}/docs`, fetcher);
    const [selectedDoc, setSelectedDoc] = useState<Doc | null>(null);
    const [isEditing, setIsEditing] = useState(false);
    const [editTitle, setEditTitle] = useState('');
    const [editContent, setEditContent] = useState('');
    const [isCreating, setIsCreating] = useState(false);
    const [newTitle, setNewTitle] = useState('');

    const handleSelectDoc = (doc: Doc) => {
        setSelectedDoc(doc);
        setIsEditing(false);
        setEditTitle(doc.title);
        setEditContent(doc.content);
    };

    const handleCreate = async () => {
        if (!newTitle.trim()) return;
        const res = await fetch(`/api/boards/${boardId}/docs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title: newTitle.trim(), content: '' }),
        });
        const created = await res.json();
        mutate(`/api/boards/${boardId}/docs`);
        setNewTitle('');
        setIsCreating(false);
        handleSelectDoc(created);
        setIsEditing(true);
    };

    const handleSave = async () => {
        if (!selectedDoc) return;
        const res = await fetch(`/api/docs/${selectedDoc.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title: editTitle, content: editContent }),
        });
        const updated = await res.json();
        mutate(`/api/boards/${boardId}/docs`);
        setSelectedDoc(updated);
        setIsEditing(false);
    };

    const handleDelete = async (docId: number) => {
        if (!confirm('Delete this document?')) return;
        await fetch(`/api/docs/${docId}`, { method: 'DELETE' });
        mutate(`/api/boards/${boardId}/docs`);
        if (selectedDoc?.id === docId) {
            setSelectedDoc(null);
        }
    };

    return (
        <div className="flex h-full overflow-hidden">
            {/* Sidebar */}
            <div className="flex w-[280px] shrink-0 flex-col border-r bg-gray-50 dark:border-zinc-800 dark:bg-zinc-900/50">
                <div className="flex items-center justify-between border-b p-3 dark:border-zinc-800">
                    <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-400">Documents</h3>
                    <button
                        onClick={() => setIsCreating(true)}
                        className="rounded-md p-1.5 text-gray-500 hover:bg-gray-200 dark:hover:bg-zinc-800"
                        title="New Document"
                    >
                        <Plus size={16} />
                    </button>
                </div>

                {isCreating && (
                    <div className="border-b p-3 dark:border-zinc-800">
                        <input
                            autoFocus
                            type="text"
                            value={newTitle}
                            onChange={(e) => setNewTitle(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') handleCreate();
                                if (e.key === 'Escape') { setIsCreating(false); setNewTitle(''); }
                            }}
                            placeholder="Document title..."
                            className="w-full rounded-lg border bg-white px-3 py-2 text-sm outline-none focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                        />
                        <div className="mt-2 flex gap-2">
                            <button onClick={handleCreate} className="rounded-md bg-rose-500 px-3 py-1 text-sm text-white hover:bg-rose-600">Create</button>
                            <button onClick={() => { setIsCreating(false); setNewTitle(''); }} className="rounded-md px-3 py-1 text-sm hover:bg-gray-200 dark:hover:bg-zinc-800">
                                <X size={14} />
                            </button>
                        </div>
                    </div>
                )}

                <div className="flex-1 overflow-y-auto">
                    {docs?.length === 0 && !isCreating && (
                        <div className="p-6 text-center text-sm text-gray-400">
                            No documents yet.<br />
                            Click <strong>+</strong> to create one.
                        </div>
                    )}
                    {docs?.map((doc) => (
                        <button
                            key={doc.id}
                            onClick={() => handleSelectDoc(doc)}
                            className={`flex w-full items-center gap-3 border-b px-3 py-3 text-left transition-colors hover:bg-gray-100 dark:border-zinc-800 dark:hover:bg-zinc-800 ${selectedDoc?.id === doc.id ? 'bg-rose-50 dark:bg-zinc-800' : ''
                                }`}
                        >
                            <FileText size={16} className="shrink-0 text-gray-400" />
                            <div className="min-w-0 flex-1">
                                <p className="truncate text-sm font-medium">{doc.title}</p>
                                <p className="text-xs text-gray-400">
                                    {new Date(doc.updated_at).toLocaleDateString()}
                                </p>
                            </div>
                        </button>
                    ))}
                </div>
            </div>

            {/* Main Content */}
            <div className="flex flex-1 flex-col overflow-hidden">
                {selectedDoc ? (
                    <>
                        {/* Doc Header */}
                        <div className="flex items-center justify-between border-b bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900">
                            <div className="flex items-center gap-3">
                                <button
                                    onClick={() => setSelectedDoc(null)}
                                    className="rounded-md p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-zinc-800 lg:hidden"
                                >
                                    <ChevronLeft size={18} />
                                </button>
                                {isEditing ? (
                                    <input
                                        type="text"
                                        value={editTitle}
                                        onChange={(e) => setEditTitle(e.target.value)}
                                        className="rounded-md border bg-gray-50 px-3 py-1 text-lg font-bold outline-none focus:border-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                                    />
                                ) : (
                                    <h2 className="text-lg font-bold">{selectedDoc.title}</h2>
                                )}
                            </div>
                            <div className="flex items-center gap-2">
                                {isEditing ? (
                                    <>
                                        <button onClick={handleSave} className="flex items-center gap-1 rounded-md bg-rose-500 px-3 py-1.5 text-sm text-white hover:bg-rose-600">
                                            <Save size={14} /> Save
                                        </button>
                                        <button onClick={() => { setIsEditing(false); setEditTitle(selectedDoc.title); setEditContent(selectedDoc.content); }} className="rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 dark:hover:bg-zinc-800">
                                            Cancel
                                        </button>
                                    </>
                                ) : (
                                    <button onClick={() => setIsEditing(true)} className="rounded-md bg-gray-100 px-3 py-1.5 text-sm font-medium hover:bg-gray-200 dark:bg-zinc-800 dark:hover:bg-zinc-700">
                                        Edit
                                    </button>
                                )}
                                <button
                                    onClick={() => handleDelete(selectedDoc.id)}
                                    className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        </div>

                        {/* Doc Content */}
                        <div className="flex-1 overflow-y-auto p-6">
                            {isEditing ? (
                                <textarea
                                    value={editContent}
                                    onChange={(e) => setEditContent(e.target.value)}
                                    placeholder="Write your documentation here (Markdown supported)..."
                                    className="h-full min-h-[400px] w-full resize-none rounded-lg border bg-gray-50 p-4 font-mono text-sm outline-none focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                                />
                            ) : (
                                <div className="prose prose-sm max-w-none dark:prose-invert">
                                    {selectedDoc.content ? (
                                        <pre className="whitespace-pre-wrap font-sans">{selectedDoc.content}</pre>
                                    ) : (
                                        <p className="text-gray-400 italic">No content yet. Click Edit to start writing.</p>
                                    )}
                                </div>
                            )}
                        </div>
                    </>
                ) : (
                    <div className="flex flex-1 flex-col items-center justify-center text-gray-400">
                        <FileText size={48} className="mb-4 opacity-30" />
                        <p className="text-lg font-medium">Select a document</p>
                        <p className="mt-1 text-sm">or create a new one from the sidebar</p>
                    </div>
                )}
            </div>
        </div>
    );
}
