import React, { useEffect, useState, useRef } from 'react';
import { Task } from './Board';
import { X, Send, User, MessageCircle, PackageCheck, History, LayoutList, AlertTriangle, Pencil, Eye } from 'lucide-react';
import useSWR, { mutate } from 'swr';
import ReactMarkdown from 'react-markdown';
import { RichTextEditor } from './ui/rich-text-editor';
import { Drawer, DrawerContent } from './ui/drawer';

interface Activity {
  id: number;
  task_id: number;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
}

interface Deliverable {
  id: number;
  task_id: number;
  title: string;
  description: string;
  artifact_type: string;
  content: string;
  created_at: string;
  updated_at: string;
}

interface Comment {
  id: number;
  task_id: number;
  user_id: string;
  content: string;
  created_at: string;
}

interface Member {
  id: string;
  name: string;
  role: string;
  avatar: string;
}

interface TaskDetailModalProps {
  task: Task;
  isOpen: boolean;
  onClose: () => void;
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

function statusTone(task: Task) {
  const status = (task.status || task.list_id || '').toLowerCase();
  if (status === 'blocked') return 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-300';
  if (status === 'review' || status === 'qa') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300';
  if (status === 'doing' || status === 'in_progress') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300';
  if (status === 'done') return 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-300';
  return 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300';
}

export function TaskDetailModal({ task, isOpen, onClose }: TaskDetailModalProps) {
  const [description, setDescription] = useState(task.description || '');
  const [commentInput, setCommentInput] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [assigneeId, setAssigneeId] = useState(task.assignee_id || '');
  const [boardId, setBoardId] = useState(task.board_id || '');
  const [activeTab, setActiveTab] = useState<'overview' | 'discussion' | 'deliverables' | 'activity'>('overview');
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const chatEndRef = useRef<HTMLDivElement>(null);

  const { data: boards } = useSWR<any[]>('/api/boards', fetcher);
  const { data: members } = useSWR<Member[]>(task.board_id ? `/api/boards/${task.board_id}/members` : null, fetcher);
  const { data: activities } = useSWR<Activity[]>(task.id ? `/api/tasks/${task.id}/activities` : null, fetcher);
  const { data: deliverables } = useSWR<Deliverable[]>(task.id ? `/api/tasks/${task.id}/deliverables` : null, fetcher);
  const { data: comments } = useSWR<Comment[]>(task.id ? `/api/tasks/${task.id}/comments` : null, fetcher, { refreshInterval: 5000 });
  const { data: session } = useSWR<{ id: string }>('/api/auth/me', fetcher);

  useEffect(() => {
    setDescription(task.description || '');
    setAssigneeId(task.assignee_id || '');
    setBoardId(task.board_id || '');
    setActiveTab('overview');
    setIsEditingDescription(false);
  }, [task]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [comments]);

  const getMember = (userId: string): Member | undefined => members?.find((m) => m.id === userId);
  const assignee = members?.find((m) => m.id === assigneeId);

  const handleSave = async () => {
    await fetch(`/api/tasks/${task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...task, description, assignee_id: assigneeId || null, board_id: boardId }),
    });
    mutate(`/api/boards/${task.board_id}/tasks`);
    if (boardId !== task.board_id) mutate(`/api/boards/${boardId}/tasks`);
    onClose();
  };

  const handlePostComment = async () => {
    if (!commentInput.trim() || isSending) return;
    setIsSending(true);
    try {
      await fetch(`/api/tasks/${task.id}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: session?.id || 'human_user', content: commentInput.trim() }),
      });
      setCommentInput('');
      mutate(`/api/tasks/${task.id}/comments`);
    } finally {
      setIsSending(false);
    }
  };

  const tabs = [
    { key: 'overview', label: 'Overview', icon: LayoutList },
    { key: 'discussion', label: 'Discussion', icon: MessageCircle },
    { key: 'deliverables', label: 'Deliverables', icon: PackageCheck },
    { key: 'activity', label: 'Activity', icon: History },
  ] as const;

  return (
    <Drawer open={isOpen} onOpenChange={(open) => !open && onClose()} direction="right">
      <DrawerContent className="data-[vaul-drawer-direction=right]:w-[92vw] data-[vaul-drawer-direction=right]:sm:max-w-3xl">
        <div className="flex h-full min-h-0 flex-col overflow-hidden">
          <div className="border-b p-5 dark:border-zinc-800">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <span className={`rounded-full px-2 py-1 text-xs font-semibold uppercase tracking-wide ${statusTone(task)}`}>
                    {task.status || task.list_id}
                  </span>
                  {task.blocked_reason && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-1 text-xs font-semibold text-red-700 dark:bg-red-900/20 dark:text-red-300">
                      <AlertTriangle size={12} /> Blocked
                    </span>
                  )}
                </div>
                <h2 className="text-2xl font-bold leading-tight">{task.title}</h2>
                <div className="mt-3 flex flex-wrap gap-3 text-sm text-muted-foreground">
                  <span>Task #{task.id}</span>
                  <span>Board: {boardId}</span>
                  <span>Assignee: {assignee ? `${assignee.avatar} ${assignee.name}` : 'Unassigned'}</span>
                </div>
              </div>
              <button onClick={onClose} className="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-zinc-800"><X size={20} /></button>
            </div>
          </div>

          <div className="flex border-b dark:border-zinc-800 overflow-x-auto">
            {tabs.map((tab) => {
              const Icon = tab.icon;
              return (
                <button
                  key={tab.key}
                  onClick={() => setActiveTab(tab.key)}
                  className={`flex items-center gap-2 whitespace-nowrap px-4 py-3 text-sm font-medium transition-colors ${activeTab === tab.key ? 'border-b-2 border-rose-500 text-rose-500' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
                >
                  <Icon size={14} /> {tab.label}
                  {tab.key === 'discussion' && !!comments?.length && <span className="rounded-full bg-rose-100 px-1.5 py-0.5 text-xs text-rose-600 dark:bg-rose-900/30 dark:text-rose-400">{comments.length}</span>}
                  {tab.key === 'deliverables' && !!deliverables?.length && <span className="rounded-full bg-blue-100 px-1.5 py-0.5 text-xs text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">{deliverables.length}</span>}
                </button>
              );
            })}
          </div>

          <div className="flex min-h-0 flex-1 overflow-hidden">
            {activeTab === 'overview' && (
              <div className="grid min-h-0 flex-1 gap-6 overflow-y-auto p-6 xl:grid-cols-[1.2fr_0.8fr]">
                <div className="space-y-6">
                  <section className="rounded-2xl border p-4 dark:border-zinc-800">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold">Description / Context</div>
                      <div className="flex items-center gap-2">
                        {isEditingDescription ? (
                          <>
                            <button
                              type="button"
                              onClick={() => {
                                setDescription(task.description || '');
                                setIsEditingDescription(false);
                              }}
                              className="rounded-lg border px-3 py-1.5 text-xs font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
                            >
                              Cancel
                            </button>
                            <button
                              type="button"
                              onClick={() => setIsEditingDescription(false)}
                              className="inline-flex items-center gap-1 rounded-lg bg-black px-3 py-1.5 text-xs font-medium text-white hover:bg-zinc-800 dark:bg-white dark:text-black dark:hover:bg-zinc-200"
                            >
                              <Eye size={12} /> Done editing
                            </button>
                          </>
                        ) : (
                          <button
                            type="button"
                            onClick={() => setIsEditingDescription(true)}
                            className="inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
                          >
                            <Pencil size={12} /> Edit
                          </button>
                        )}
                      </div>
                    </div>
                    <div className="min-h-[260px]">
                      {isEditingDescription ? (
                        <RichTextEditor content={description} onChange={setDescription} placeholder="Add details..." />
                      ) : (
                        <div className="min-h-[260px] rounded-2xl border bg-zinc-50/70 p-5 dark:border-zinc-800 dark:bg-zinc-950/40">
                          {description?.trim() ? (
                            <div className="prose prose-sm max-w-none dark:prose-invert break-words">
                              <ReactMarkdown>{description.replace(/<br\s*\/?>/gi, '\n').replace(/<[^>]*>/g, '')}</ReactMarkdown>
                            </div>
                          ) : (
                            <div className="flex h-[220px] flex-col items-center justify-center text-center text-muted-foreground">
                              <LayoutList size={28} className="mb-3 opacity-30" />
                              <div className="text-sm font-medium">No description yet</div>
                              <div className="mt-1 text-xs">Add context, goals, specs, or acceptance criteria for this task.</div>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </section>

                  {task.blocked_reason && (
                    <section className="rounded-2xl border border-red-200 bg-red-50 p-4 dark:border-red-900/30 dark:bg-red-900/10">
                      <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-red-700 dark:text-red-300">
                        <AlertTriangle size={16} /> Blocked Reason
                      </div>
                      <div className="text-sm text-red-700/90 dark:text-red-200">{task.blocked_reason}</div>
                    </section>
                  )}
                </div>

                <div className="space-y-4">
                  <section className="rounded-2xl border p-4 dark:border-zinc-800">
                    <div className="mb-3 text-sm font-semibold">Task Metadata</div>
                    <div className="space-y-4">
                      <MetaField label="Assignee">
                        <select value={assigneeId} onChange={(e) => setAssigneeId(e.target.value)} className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700">
                          <option value="">Unassigned</option>
                          {members?.map((m) => <option key={m.id} value={m.id}>{m.avatar} {m.name} ({m.role})</option>)}
                        </select>
                      </MetaField>
                      <MetaField label="Board">
                        <select value={boardId} onChange={(e) => setBoardId(e.target.value)} className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none focus:border-rose-500 dark:border-zinc-700">
                          {boards?.map((b) => <option key={b.id} value={b.id}>{b.title}</option>)}
                        </select>
                      </MetaField>
                    </div>
                  </section>

                  <section className="rounded-2xl border p-4 dark:border-zinc-800">
                    <div className="mb-3 text-sm font-semibold">Quick Signals</div>
                    <div className="grid grid-cols-2 gap-3 text-sm">
                      <SignalCard label="Comments" value={String(comments?.length || 0)} />
                      <SignalCard label="Deliverables" value={String(deliverables?.length || 0)} />
                      <SignalCard label="Activity" value={String(activities?.length || 0)} />
                      <SignalCard label="Status" value={task.status || task.list_id} />
                    </div>
                  </section>
                </div>
              </div>
            )}

            {activeTab === 'discussion' && (
              <div className="flex min-h-0 flex-1 flex-col">
                <div className="flex-1 overflow-y-auto p-5 space-y-4">
                  {(!comments || comments.length === 0) && (
                    <div className="flex h-full flex-col items-center justify-center text-gray-400">
                      <MessageCircle size={40} className="mb-3 opacity-30" />
                      <p className="text-sm font-medium">No discussion yet</p>
                      <p className="mt-1 text-xs">Use this thread to coordinate with humans and agents.</p>
                    </div>
                  )}
                  {comments?.map((cm) => {
                    const member = getMember(cm.user_id);
                    const isHuman = member?.role === 'human';
                    return (
                      <div key={cm.id} className={`flex gap-3 ${isHuman ? 'flex-row-reverse' : ''}`}>
                        <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm ${isHuman ? 'bg-rose-100 dark:bg-rose-900/30' : 'bg-blue-100 dark:bg-blue-900/30'}`}>
                          {member?.avatar || '🤖'}
                        </div>
                        <div className={`max-w-[78%] rounded-2xl px-4 py-3 ${isHuman ? 'bg-rose-500 text-white' : 'bg-gray-100 text-gray-900 dark:bg-zinc-800 dark:text-gray-100'}`}>
                          <div className={`mb-1 flex items-center gap-2 text-xs ${isHuman ? 'text-rose-200' : 'text-gray-500 dark:text-gray-400'}`}>
                            <span className="font-semibold">{member?.name || cm.user_id}</span>
                            {member?.role === 'agent' && <span className="rounded bg-blue-200/20 px-1 py-0.5 text-[10px] font-bold uppercase text-blue-300">AI</span>}
                          </div>
                          <p className="text-sm whitespace-pre-wrap">{cm.content}</p>
                          <div className={`mt-2 text-[10px] ${isHuman ? 'text-rose-200' : 'text-gray-400'}`}>{new Date(cm.created_at).toLocaleString()}</div>
                        </div>
                      </div>
                    );
                  })}
                  <div ref={chatEndRef} />
                </div>
                <div className="border-t p-4 dark:border-zinc-800">
                  <div className="relative">
                    <input type="text" value={commentInput} onChange={(e) => setCommentInput(e.target.value)} placeholder="Write an update, ask for review, or leave context..." className="w-full rounded-2xl border bg-gray-50 py-3 pl-4 pr-12 text-sm outline-none focus:ring-2 focus:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-800" onKeyDown={(e) => e.key === 'Enter' && handlePostComment()} />
                    <button onClick={handlePostComment} disabled={isSending || !commentInput.trim()} className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full bg-rose-500 p-2 text-white hover:bg-rose-600 disabled:opacity-50"><Send size={14} /></button>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'deliverables' && (
              <div className="flex-1 overflow-y-auto p-6">
                {deliverables && deliverables.length > 0 ? (
                  <div className="space-y-4">
                    {deliverables.map((d) => (
                      <div key={d.id} className="rounded-2xl border bg-gray-50 p-4 dark:border-zinc-700 dark:bg-zinc-800">
                        <div className="mb-1 font-semibold text-sm">{d.title}</div>
                        <div className="text-xs text-gray-500 mb-3">{d.description}</div>
                        {d.content && <div className="rounded-xl border bg-white p-3 text-sm dark:border-zinc-700 dark:bg-zinc-900"><pre className="whitespace-pre-wrap font-sans">{d.content}</pre></div>}
                        <div className="mt-3 text-[10px] text-gray-400 text-right">{new Date(d.created_at).toLocaleString()}</div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <EmptyState icon={PackageCheck} title="No deliverables yet" subtitle="Agent outputs, files, and summaries will appear here." />
                )}
              </div>
            )}

            {activeTab === 'activity' && (
              <div className="flex-1 overflow-y-auto p-6">
                {activities && activities.length > 0 ? (
                  <div className="space-y-3">
                    {activities.map((act) => (
                      <div key={act.id} className="rounded-xl border p-4 dark:border-zinc-800">
                        <div className="text-sm"><span className="font-semibold">{act.user_id}</span> <span className="text-gray-600 dark:text-gray-400">{act.details}</span></div>
                        <div className="mt-1 text-xs text-gray-400">{new Date(act.created_at).toLocaleString()}</div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <EmptyState icon={History} title="No activity yet" subtitle="Task movement, system updates, and agent actions will show up here." />
                )}
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2 border-t p-4 dark:border-zinc-800">
            <button onClick={onClose} className="rounded-lg px-4 py-2 hover:bg-gray-100 dark:hover:bg-zinc-800">Cancel</button>
            <button onClick={handleSave} className="rounded-lg bg-black px-6 py-2 text-white hover:bg-gray-800 dark:bg-white dark:text-black">Save</button>
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  );
}

function MetaField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</div>
      {children}
    </div>
  );
}

function SignalCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border bg-white p-3 text-center dark:border-zinc-700 dark:bg-zinc-900">
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="mt-1 text-base font-semibold">{value}</div>
    </div>
  );
}

function EmptyState({ icon: Icon, title, subtitle }: { icon: React.ComponentType<any>; title: string; subtitle: string }) {
  return (
    <div className="flex h-full min-h-[320px] flex-col items-center justify-center text-gray-400">
      <Icon size={40} className="mb-3 opacity-30" />
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-1 text-xs">{subtitle}</p>
    </div>
  );
}
