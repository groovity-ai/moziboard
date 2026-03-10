import React, { useState } from 'react';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useDroppable } from '@dnd-kit/core';
import { TaskCard } from './TaskCard';
import { ListType, Task } from './Board';
import { mutate } from 'swr';
import { AlertTriangle, CheckCircle2, CircleDot, Eye, Inbox, Play, Plus, X } from 'lucide-react';
import clsx from 'clsx';

interface ListContainerProps {
  list: ListType;
  boardId: string;
  onTaskClick?: (task: Task) => void;
  isActiveDropTarget?: boolean;
}

const listSemantics: Record<string, { icon: React.ComponentType<any>; tone: string; badge: string }> = {
  backlog: { icon: Inbox, tone: 'text-zinc-500', badge: 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300' },
  todo: { icon: CircleDot, tone: 'text-sky-500', badge: 'bg-sky-100 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300' },
  doing: { icon: Play, tone: 'text-blue-500', badge: 'bg-blue-100 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300' },
  qa: { icon: Eye, tone: 'text-amber-500', badge: 'bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300' },
  done: { icon: CheckCircle2, tone: 'text-green-500', badge: 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-300' },
  blocked: { icon: AlertTriangle, tone: 'text-red-500', badge: 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-300' },
};

export function ListContainer({ list, boardId, onTaskClick, isActiveDropTarget = false }: ListContainerProps) {
  const [isAdding, setIsAdding] = useState(false);
  const [newTitle, setNewTitle] = useState('');

  const { setNodeRef, isOver } = useDroppable({ id: list.id });
  const semantic = listSemantics[list.id] || listSemantics.todo;
  const Icon = semantic.icon;

  const handleAddTask = async () => {
    if (!newTitle.trim()) return;

    await fetch('/api/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: newTitle.trim(),
        description: '',
        list_id: list.id,
        board_id: boardId,
        position: list.tasks.length + 1,
      }),
    });
    mutate(`/api/boards/${boardId}/tasks`);
    setNewTitle('');
    setIsAdding(false);
  };

  return (
    <div
      ref={setNodeRef}
      className={clsx(
        'flex h-fit w-[350px] shrink-0 flex-col gap-4 rounded-2xl border bg-gray-100 p-4 shadow-sm transition-all dark:bg-zinc-900',
        (isOver || isActiveDropTarget)
          ? 'border-rose-400 ring-2 ring-rose-200 dark:border-rose-500 dark:ring-rose-900/30'
          : 'border-transparent'
      )}
    >
      <div className="sticky top-0 z-10 flex items-center justify-between rounded-xl bg-white/70 px-2 py-2 backdrop-blur dark:bg-zinc-950/40">
        <div className="flex items-center gap-2">
          <Icon size={16} className={semantic.tone} />
          <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${semantic.badge}`}>{list.title}</span>
        </div>
        <span className="rounded-full bg-gray-200 px-2 py-1 text-sm dark:bg-zinc-800">
          {list.tasks.length}
        </span>
      </div>

      <div className="flex min-h-[120px] flex-col gap-2">
        <SortableContext items={list.tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
          {list.tasks.map((task) => (
            <TaskCard key={task.id} task={task} onClick={() => onTaskClick?.(task)} />
          ))}
        </SortableContext>
        {list.tasks.length === 0 && (
          <div className="flex min-h-[120px] flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 bg-white/60 text-sm text-gray-400 dark:border-zinc-700 dark:bg-zinc-800/40 dark:text-zinc-500">
            <Icon size={18} className="mb-2 opacity-60" />
            <div className="font-medium">Empty</div>
            <div className="mt-1 text-xs">Drop task here or create a new one</div>
          </div>
        )}
      </div>

      {isAdding ? (
        <div className="flex flex-col gap-2">
          <input
            autoFocus
            type="text"
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleAddTask();
              if (e.key === 'Escape') { setIsAdding(false); setNewTitle(''); }
            }}
            placeholder="Enter task title..."
            className="w-full rounded-lg border bg-white px-3 py-2 text-sm outline-none focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-800"
          />
          <div className="flex gap-2">
            <button onClick={handleAddTask} className="rounded-md bg-rose-500 px-3 py-1 text-sm text-white hover:bg-rose-600">Add</button>
            <button onClick={() => { setIsAdding(false); setNewTitle(''); }} className="rounded-md px-3 py-1 text-sm hover:bg-gray-200 dark:hover:bg-zinc-800"><X size={16} /></button>
          </div>
        </div>
      ) : (
        <button onClick={() => setIsAdding(true)} className="flex w-full items-center gap-2 rounded-md p-2 text-sm hover:bg-gray-200 dark:hover:bg-zinc-800">
          <Plus size={16} /> Add Task
        </button>
      )}
    </div>
  );
}
