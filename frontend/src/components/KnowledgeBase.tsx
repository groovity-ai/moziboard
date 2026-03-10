'use client';

import React, { useEffect, useMemo, useState } from 'react';
import useSWR, { mutate } from 'swr';
import { useSearchParams } from 'next/navigation';
import {
  Plus,
  FileText,
  Trash2,
  Save,
  X,
  Search,
  Sparkles,
  PenSquare,
  CalendarClock,
  BookOpen,
  PanelLeftOpen,
} from 'lucide-react';

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

function formatDate(value?: string) {
  if (!value) return '-';
  try {
    return new Date(value).toLocaleString();
  } catch {
    return value;
  }
}

function stripMarkdown(text?: string) {
  return (text || '')
    .replace(/[#>*_`~\-]/g, ' ')
    .replace(/\[(.*?)\]\((.*?)\)/g, '$1')
    .replace(/\s+/g, ' ')
    .trim();
}

export function KnowledgeBase({ boardId }: KnowledgeBaseProps) {
  const searchParams = useSearchParams();
  const requestedDocId = searchParams.get('doc');
  const { data: docs, isLoading } = useSWR<Doc[]>(`/api/boards/${boardId}/docs`, fetcher, {
    revalidateOnFocus: false,
  });

  const [selectedDoc, setSelectedDoc] = useState<Doc | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [editContent, setEditContent] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [filterQuery, setFilterQuery] = useState('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');

  useEffect(() => {
    if (!docs?.length) {
      setSelectedDoc(null);
      return;
    }

    if (requestedDocId) {
      const matched = docs.find((doc) => String(doc.id) === requestedDocId);
      if (matched) {
        setSelectedDoc(matched);
        setEditTitle(matched.title);
        setEditContent(matched.content || '');
        return;
      }
    }

    if (!selectedDoc) {
      setSelectedDoc(docs[0]);
      setEditTitle(docs[0].title);
      setEditContent(docs[0].content || '');
    } else {
      const fresh = docs.find((doc) => doc.id === selectedDoc.id);
      if (fresh) {
        setSelectedDoc(fresh);
        if (!isEditing) {
          setEditTitle(fresh.title);
          setEditContent(fresh.content || '');
        }
      }
    }
  }, [docs, requestedDocId]);

  const filteredDocs = useMemo(() => {
    const source = docs || [];
    if (!filterQuery.trim()) return source;
    const q = filterQuery.toLowerCase();
    return source.filter((doc) =>
      doc.title.toLowerCase().includes(q) || stripMarkdown(doc.content).toLowerCase().includes(q)
    );
  }, [docs, filterQuery]);

  const stats = useMemo(() => {
    const allDocs = docs || [];
    const totalWords = allDocs.reduce((acc, doc) => acc + stripMarkdown(doc.content).split(' ').filter(Boolean).length, 0);
    return {
      total: allDocs.length,
      words: totalWords,
      lastUpdated: allDocs[0]?.updated_at,
    };
  }, [docs]);

  const handleSelectDoc = (doc: Doc) => {
    setSelectedDoc(doc);
    setIsEditing(false);
    setEditTitle(doc.title);
    setEditContent(doc.content || '');
  };

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    const res = await fetch(`/api/boards/${boardId}/docs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: newTitle.trim(), content: '' }),
    });
    const created = await res.json();
    await mutate(`/api/boards/${boardId}/docs`);
    setNewTitle('');
    setIsCreating(false);
    handleSelectDoc(created);
    setIsEditing(true);
  };

  const handleSave = async () => {
    if (!selectedDoc) return;
    setSaveState('saving');
    try {
      const res = await fetch(`/api/docs/${selectedDoc.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: editTitle, content: editContent }),
      });
      const updated = await res.json();
      await mutate(`/api/boards/${boardId}/docs`);
      setSelectedDoc(updated);
      setIsEditing(false);
      setSaveState('saved');
    } catch {
      setSaveState('error');
    }
  };

  const handleDelete = async (docId: number) => {
    if (!confirm('Delete this document?')) return;
    await fetch(`/api/docs/${docId}`, { method: 'DELETE' });
    await mutate(`/api/boards/${boardId}/docs`);
    if (selectedDoc?.id === docId) {
      setSelectedDoc(null);
    }
  };

  return (
    <div className="flex h-full overflow-hidden bg-gradient-to-br from-background via-background to-emerald-50/30 dark:to-zinc-950">
      <aside
        className={`$${''}{{ sidebarOpen ? 'flex' : 'hidden md:flex' }} w-[320px] shrink-0 flex-col border-r bg-white/80 backdrop-blur dark:border-zinc-800 dark:bg-zinc-900/70`}
      >
        <div className="border-b p-4 dark:border-zinc-800">
          <div className="mb-4 rounded-2xl border bg-gradient-to-br from-emerald-500/8 to-teal-500/6 p-4 dark:border-zinc-800 dark:from-emerald-500/10 dark:to-teal-500/10">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <BookOpen className="h-4 w-4 text-emerald-500" />
                  Knowledge Base
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                  Living docs for specs, context, decisions, and operational notes.
                </p>
              </div>
              <Sparkles className="h-4 w-4 text-emerald-500" />
            </div>

            <div className="mt-4 grid grid-cols-3 gap-2">
              <div className="rounded-xl bg-background/80 p-2 text-center dark:bg-zinc-950/70">
                <div className="text-lg font-semibold">{stats.total}</div>
                <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Docs</div>
              </div>
              <div className="rounded-xl bg-background/80 p-2 text-center dark:bg-zinc-950/70">
                <div className="text-lg font-semibold">{stats.words}</div>
                <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Words</div>
              </div>
              <div className="rounded-xl bg-background/80 p-2 text-center dark:bg-zinc-950/70">
                <div className="text-lg font-semibold">{filteredDocs.length}</div>
                <div className="text-[10px] uppercase tracking-wide text-muted-foreground">Visible</div>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <input
                type="text"
                value={filterQuery}
                onChange={(e) => setFilterQuery(e.target.value)}
                placeholder="Filter docs in this page..."
                className="h-10 w-full rounded-xl border bg-background pl-9 pr-3 text-sm outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-zinc-800 dark:bg-zinc-950"
              />
            </div>
            <button
              onClick={() => setIsCreating(true)}
              className="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500 text-white shadow-sm transition hover:bg-emerald-600"
              title="New document"
            >
              <Plus size={18} />
            </button>
          </div>

          {isCreating && (
            <div className="mt-3 rounded-2xl border bg-background p-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-950">
              <input
                autoFocus
                type="text"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleCreate();
                  if (e.key === 'Escape') {
                    setIsCreating(false);
                    setNewTitle('');
                  }
                }}
                placeholder="New document title..."
                className="w-full rounded-lg border bg-background px-3 py-2 text-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-zinc-800 dark:bg-zinc-900"
              />
              <div className="mt-3 flex gap-2">
                <button onClick={handleCreate} className="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-600">Create</button>
                <button onClick={() => { setIsCreating(false); setNewTitle(''); }} className="rounded-lg px-3 py-1.5 text-sm hover:bg-muted">Cancel</button>
              </div>
            </div>
          )}
        </div>

        <div className="flex-1 overflow-y-auto p-3">
          {isLoading && <div className="p-4 text-sm text-muted-foreground">Loading docs...</div>}
          {!isLoading && filteredDocs.length === 0 && (
            <div className="flex min-h-[220px] flex-col items-center justify-center rounded-2xl border border-dashed text-center text-sm text-muted-foreground dark:border-zinc-800">
              <FileText className="mb-3 h-8 w-8 opacity-40" />
              <div className="font-medium">No documents found</div>
              <div className="mt-1 max-w-[220px] text-xs">Create your first document or change the current filter.</div>
            </div>
          )}

          <div className="space-y-2">
            {filteredDocs.map((doc) => {
              const snippet = stripMarkdown(doc.content).slice(0, 88) || 'No content yet';
              const selected = selectedDoc?.id === doc.id;
              return (
                <button
                  key={doc.id}
                  onClick={() => handleSelectDoc(doc)}
                  className={`w-full rounded-2xl border p-3 text-left transition ${selected
                    ? 'border-emerald-500/40 bg-emerald-50 shadow-sm dark:bg-emerald-500/10'
                    : 'bg-background/80 hover:border-emerald-500/20 hover:bg-white dark:border-zinc-800 dark:bg-zinc-950/70 dark:hover:bg-zinc-900'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <div className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${selected ? 'bg-emerald-500 text-white' : 'bg-muted text-muted-foreground'}`}>
                      <FileText size={16} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-semibold">{doc.title}</div>
                      <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{snippet}</div>
                      <div className="mt-2 flex items-center gap-1 text-[10px] uppercase tracking-wide text-muted-foreground">
                        <CalendarClock className="h-3 w-3" />
                        {new Date(doc.updated_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      </aside>

      <section className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="border-b bg-white/70 px-5 py-3 backdrop-blur dark:border-zinc-800 dark:bg-zinc-900/60">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <button
                onClick={() => setSidebarOpen((v) => !v)}
                className="inline-flex h-9 w-9 items-center justify-center rounded-xl border bg-background hover:bg-muted dark:border-zinc-800 dark:bg-zinc-950"
                title="Toggle documents panel"
              >
                <PanelLeftOpen size={16} />
              </button>
              <div>
                <div className="text-[10px] font-medium uppercase tracking-[0.2em] text-muted-foreground">Documentation Workspace</div>
                <div className="text-sm text-muted-foreground">
                  {saveState === 'saving' && 'Saving document...'}
                  {saveState === 'saved' && 'Document saved'}
                  {saveState === 'error' && 'Save failed'}
                  {saveState === 'idle' && (selectedDoc ? 'Review, write, and maintain board knowledge.' : 'Pick a document to begin.')}
                </div>
              </div>
            </div>

            {selectedDoc && (
              <div className="flex items-center gap-2">
                {isEditing ? (
                  <>
                    <button onClick={handleSave} className="inline-flex items-center gap-2 rounded-xl bg-emerald-500 px-3 py-2 text-sm font-medium text-white hover:bg-emerald-600">
                      <Save size={14} /> Save
                    </button>
                    <button
                      onClick={() => {
                        setIsEditing(false);
                        setEditTitle(selectedDoc.title);
                        setEditContent(selectedDoc.content || '');
                      }}
                      className="rounded-xl border px-3 py-2 text-sm hover:bg-muted dark:border-zinc-800"
                    >
                      Cancel
                    </button>
                  </>
                ) : (
                  <button onClick={() => setIsEditing(true)} className="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm hover:bg-muted dark:border-zinc-800">
                    <PenSquare size={14} /> Edit
                  </button>
                )}
                <button
                  onClick={() => selectedDoc && handleDelete(selectedDoc.id)}
                  className="inline-flex h-10 w-10 items-center justify-center rounded-xl border text-muted-foreground hover:border-red-500/30 hover:bg-red-50 hover:text-red-500 dark:border-zinc-800 dark:hover:bg-red-500/10"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {selectedDoc ? (
            <div className="mx-auto flex h-full max-w-5xl flex-col gap-6">
              <div className="rounded-3xl border bg-white/80 p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900/70">
                {isEditing ? (
                  <input
                    type="text"
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    className="w-full border-none bg-transparent p-0 text-3xl font-bold outline-none placeholder:text-muted-foreground"
                    placeholder="Document title"
                  />
                ) : (
                  <h2 className="text-3xl font-bold tracking-tight">{selectedDoc.title}</h2>
                )}

                <div className="mt-4 flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <span className="rounded-full bg-muted px-3 py-1 dark:bg-zinc-800">Doc #{selectedDoc.id}</span>
                  <span className="rounded-full bg-muted px-3 py-1 dark:bg-zinc-800">Updated {formatDate(selectedDoc.updated_at)}</span>
                  <span className="rounded-full bg-muted px-3 py-1 dark:bg-zinc-800">Created {formatDate(selectedDoc.created_at)}</span>
                </div>
              </div>

              <div className="flex-1 rounded-3xl border bg-white/80 p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900/70">
                {isEditing ? (
                  <textarea
                    value={editContent}
                    onChange={(e) => setEditContent(e.target.value)}
                    placeholder="Write documentation, decisions, SOP, runbook, specs, or internal context here..."
                    className="min-h-[520px] w-full resize-none rounded-2xl border bg-background p-4 font-mono text-sm leading-7 outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 dark:border-zinc-800 dark:bg-zinc-950"
                  />
                ) : (
                  <div className="prose prose-zinc max-w-none dark:prose-invert">
                    {selectedDoc.content ? (
                      <pre className="whitespace-pre-wrap rounded-2xl bg-transparent p-0 font-sans text-sm leading-7">{selectedDoc.content}</pre>
                    ) : (
                      <div className="flex min-h-[320px] flex-col items-center justify-center rounded-2xl border border-dashed text-center text-muted-foreground dark:border-zinc-800">
                        <Sparkles className="mb-3 h-8 w-8 opacity-40" />
                        <div className="font-medium">Empty document</div>
                        <div className="mt-1 text-sm">Klik edit dan mulai nulis knowledge yang penting.</div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="flex h-full flex-col items-center justify-center text-center text-muted-foreground">
              <div className="mb-5 flex h-20 w-20 items-center justify-center rounded-3xl bg-emerald-500/10 text-emerald-500">
                <FileText size={34} />
              </div>
              <div className="text-xl font-semibold text-foreground">No document selected</div>
              <div className="mt-2 max-w-md text-sm">
                Pilih dokumen dari panel kiri atau bikin doc baru buat mulai nyusun SOP, spec, context, dan keputusan penting board ini.
              </div>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
