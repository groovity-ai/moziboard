import React, { useEffect, useState } from 'react';
import { Task } from './Board';
import { X, Send, User } from 'lucide-react';
import useSWR, { mutate } from 'swr';

interface Activity {
  id: number;
  task_id: number;
  user_id: string;
  action: string;
  details: string;
  created_at: string;
}

interface TaskDetailModalProps {
  task: Task;
  isOpen: boolean;
  onClose: () => void;
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export function TaskDetailModal({ task, isOpen, onClose }: TaskDetailModalProps) {
  const [description, setDescription] = useState(task.description || '');
  const [chatInput, setChatInput] = useState('');
  const [isAiProcessing, setIsAiProcessing] = useState(false);
  const [assigneeId, setAssigneeId] = useState(task.assignee_id || '');

  const { data: members } = useSWR(task.board_id ? `/api/boards/${task.board_id}/members` : null, fetcher);
  const { data: activities } = useSWR(task.id ? `/api/tasks/${task.id}/activities` : null, fetcher);

  useEffect(() => {
    setDescription(task.description || '');
    setAssigneeId(task.assignee_id || '');
  }, [task]);

  if (!isOpen) return null;

  const handleSave = async () => {
    await fetch(`/api/tasks/${task.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...task, description, assignee_id: assigneeId || null }),
    });
    mutate('/api/tasks');
    onClose();
  };

  const handleAiChat = async () => {
    if (!chatInput.trim()) return;
    setIsAiProcessing(true);
    
    // Simulate AI thinking (replace with real agent call later)
    setTimeout(async () => {
      const newContext = `\n\n> **User**: ${chatInput}\n> **AI**: Processed. Added to context.`;
      setDescription((prev) => prev + newContext);
      setChatInput('');
      setIsAiProcessing(false);
    }, 1000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="flex h-[80vh] w-full max-w-2xl flex-col rounded-2xl bg-white shadow-2xl dark:bg-zinc-900">
        
        {/* Header */}
        <div className="flex items-center justify-between border-b p-4 dark:border-zinc-800">
          <h2 className="text-xl font-bold">{task.title}</h2>
          <button onClick={onClose} className="rounded-full p-2 hover:bg-gray-100 dark:hover:bg-zinc-800">
            <X size={20} />
          </button>
        </div>

        {/* Body */}
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
              {members?.map((m: any) => (
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
              {activities?.length > 0 ? (
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

          {/* AI Chat Bar */}
          <div className="relative">
            <input
              type="text"
              value={chatInput}
              onChange={(e) => setChatInput(e.target.value)}
              placeholder="Ask AI to update this task..."
              className="w-full rounded-full border bg-gray-100 py-3 pl-4 pr-12 outline-none focus:ring-2 focus:ring-rose-500 dark:bg-zinc-800 dark:border-zinc-700"
              onKeyDown={(e) => e.key === 'Enter' && handleAiChat()}
            />
            <button 
              onClick={handleAiChat}
              disabled={isAiProcessing}
              className="absolute right-2 top-2 rounded-full bg-rose-500 p-2 text-white hover:bg-rose-600 disabled:opacity-50"
            >
              <Send size={16} />
            </button>
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
      </div>
    </div>
  );
}
