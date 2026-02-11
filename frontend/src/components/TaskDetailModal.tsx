import React, { useEffect, useState, useRef } from 'react';
import { Task } from './Board';
import { X, Send, User, MessageCircle } from 'lucide-react';
import useSWR, { mutate } from 'swr';

interface Activity {
  id: number;
  task_id: number;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
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

export function TaskDetailModal({ task, isOpen, onClose }: TaskDetailModalProps) {
  const [description, setDescription] = useState(task.description || '');
  const [commentInput, setCommentInput] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [assigneeId, setAssigneeId] = useState(task.assignee_id || '');
  const [activeTab, setActiveTab] = useState<'details' | 'discussion'>('discussion');
  const chatEndRef = useRef<HTMLDivElement>(null);

  const { data: members } = useSWR<Member[]>(task.board_id ? `/api/boards/${task.board_id}/members` : null, fetcher);
  const { data: activities } = useSWR<Activity[]>(task.id ? `/api/tasks/${task.id}/activities` : null, fetcher);
  const { data: comments } = useSWR<Comment[]>(task.id ? `/api/tasks/${task.id}/comments` : null, fetcher, {
    refreshInterval: 5000,
  });

  useEffect(() => {
    setDescription(task.description || '');
    setAssigneeId(task.assignee_id || '');
  }, [task]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [comments]);

  if (!isOpen) return null;

  const getMember = (userId: string): Member | undefined => {
    return members?.find((m) => m.id === userId);
  };

  const handleSave = async () => {
    await fetch(`/api/tasks/${task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...task, description, assignee_id: assigneeId || null }),
    });
    mutate(`/api/boards/${task.board_id}/tasks`);
    onClose();
  };

  const handlePostComment = async () => {
    if (!commentInput.trim() || isSending) return;
    setIsSending(true);
    try {
      await fetch(`/api/tasks/${task.id}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: 'mirza', content: commentInput.trim() }),
      });
      setCommentInput('');
      mutate(`/api/tasks/${task.id}/comments`);
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="flex h-[85vh] w-full max-w-2xl flex-col rounded-2xl bg-white shadow-2xl dark:bg-zinc-900">

        {/* Header */}
        <div className="flex items-center justify-between border-b p-4 dark:border-zinc-800">
          <h2 className="text-xl font-bold">{task.title}</h2>
          <button onClick={onClose} className="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-zinc-800">
            <X size={20} />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b dark:border-zinc-800">
          <button
            onClick={() => setActiveTab('discussion')}
            className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium transition-colors ${activeTab === 'discussion'
                ? 'border-b-2 border-rose-500 text-rose-500'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'
              }`}
          >
            <MessageCircle size={14} />
            Discussion
            {comments && comments.length > 0 && (
              <span className="ml-1 rounded-full bg-rose-100 px-1.5 py-0.5 text-xs text-rose-600 dark:bg-rose-900/30 dark:text-rose-400">
                {comments.length}
              </span>
            )}
          </button>
          <button
            onClick={() => setActiveTab('details')}
            className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium transition-colors ${activeTab === 'details'
                ? 'border-b-2 border-rose-500 text-rose-500'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'
              }`}
          >
            <User size={14} />
            Details
          </button>
        </div>

        {/* Tab Content */}
        <div className="flex flex-1 flex-col overflow-hidden">

          {activeTab === 'discussion' ? (
            <>
              {/* Comments Thread */}
              <div className="flex-1 overflow-y-auto p-4 space-y-4">
                {(!comments || comments.length === 0) && (
                  <div className="flex h-full flex-col items-center justify-center text-gray-400">
                    <MessageCircle size={40} className="mb-3 opacity-30" />
                    <p className="text-sm font-medium">No comments yet</p>
                    <p className="mt-1 text-xs">Start the discussion below</p>
                  </div>
                )}
                {comments?.map((cm) => {
                  const member = getMember(cm.user_id);
                  const isHuman = member?.role === 'human';
                  return (
                    <div key={cm.id} className={`flex gap-3 ${isHuman ? 'flex-row-reverse' : ''}`}>
                      {/* Avatar */}
                      <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm ${isHuman
                          ? 'bg-rose-100 dark:bg-rose-900/30'
                          : 'bg-blue-100 dark:bg-blue-900/30'
                        }`}>
                        {member?.avatar || '🤖'}
                      </div>
                      {/* Bubble */}
                      <div className={`max-w-[75%] rounded-2xl px-4 py-2.5 ${isHuman
                          ? 'bg-rose-500 text-white'
                          : 'bg-gray-100 text-gray-900 dark:bg-zinc-800 dark:text-gray-100'
                        }`}>
                        <div className={`mb-1 flex items-center gap-2 text-xs ${isHuman ? 'text-rose-200' : 'text-gray-500 dark:text-gray-400'
                          }`}>
                          <span className="font-semibold">{member?.name || cm.user_id}</span>
                          {member?.role === 'agent' && (
                            <span className="rounded bg-blue-200/20 px-1 py-0.5 text-[10px] font-bold uppercase text-blue-300">
                              AI
                            </span>
                          )}
                        </div>
                        <p className="text-sm whitespace-pre-wrap">{cm.content}</p>
                        <div className={`mt-1 text-[10px] ${isHuman ? 'text-rose-200' : 'text-gray-400'
                          }`}>
                          {new Date(cm.created_at).toLocaleString()}
                        </div>
                      </div>
                    </div>
                  );
                })}
                <div ref={chatEndRef} />
              </div>

              {/* Comment Input */}
              <div className="border-t p-4 dark:border-zinc-800">
                <div className="relative">
                  <input
                    type="text"
                    value={commentInput}
                    onChange={(e) => setCommentInput(e.target.value)}
                    placeholder="Type a message..."
                    className="w-full rounded-full border bg-gray-50 py-3 pl-4 pr-12 text-sm outline-none focus:ring-2 focus:ring-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                    onKeyDown={(e) => e.key === 'Enter' && handlePostComment()}
                  />
                  <button
                    onClick={handlePostComment}
                    disabled={isSending || !commentInput.trim()}
                    className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full bg-rose-500 p-2 text-white hover:bg-rose-600 disabled:opacity-50"
                  >
                    <Send size={14} />
                  </button>
                </div>
              </div>
            </>
          ) : (
            <>
              {/* Details Tab */}
              <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-6">

                {/* Assignee Selector */}
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2 text-sm font-medium text-gray-500">
                    <User size={16} /> Assignee:
                  </div>
                  <select
                    value={assigneeId}
                    onChange={(e) => setAssigneeId(e.target.value)}
                    className="rounded-md border bg-transparent px-3 py-1 text-sm outline-none focus:border-rose-500 dark:border-zinc-700"
                  >
                    <option value="">Unassigned</option>
                    {members?.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.avatar} {m.name} ({m.role})
                      </option>
                    ))}
                  </select>
                </div>

                <div className="flex-1">
                  <label className="mb-2 block text-sm font-medium text-gray-500">Description / Context</label>
                  <textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    className="h-full w-full min-h-[200px] resize-none rounded-xl border bg-gray-50 p-4 outline-none focus:ring-2 focus:ring-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
                    placeholder="Add details..."
                  />
                </div>

                {/* Activity Log */}
                <div>
                  <label className="mb-2 block text-sm font-medium text-gray-500">Activity</label>
                  <div className="max-h-[150px] overflow-y-auto rounded-lg border bg-gray-50 p-3 text-sm dark:bg-zinc-800 dark:border-zinc-700">
                    {activities && activities.length > 0 ? (
                      activities.map((act: Activity) => (
                        <div key={act.id} className="mb-2 last:mb-0">
                          <span className="font-bold">{act.user_id}</span> <span className="text-gray-600 dark:text-gray-400">{act.details}</span>
                          <div className="text-xs text-gray-400">{new Date(act.created_at).toLocaleString()}</div>
                        </div>
                      ))
                    ) : (
                      <div className="text-gray-400 text-xs italic">No activity yet.</div>
                    )}
                  </div>
                </div>
              </div>

              {/* Footer */}
              <div className="flex justify-end gap-2 border-t p-4 dark:border-zinc-800">
                <button onClick={onClose} className="rounded-lg px-4 py-2 hover:bg-gray-100 dark:hover:bg-zinc-800">
                  Cancel
                </button>
                <button onClick={handleSave} className="rounded-lg bg-black px-6 py-2 text-white hover:bg-gray-800 dark:bg-white dark:text-black">
                  Save
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
