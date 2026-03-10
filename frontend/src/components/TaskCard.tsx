import React from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Task } from './Board';
import clsx from 'clsx';
import { AlertTriangle, GripVertical, User, Hash } from 'lucide-react';
import useSWR from 'swr';

interface TaskCardProps {
  task: Task;
  onClick?: () => void;
  dragOnly?: boolean;
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

export function TaskCard({ task, onClick, dragOnly = false }: TaskCardProps) {
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: task.id,
    data: {
      type: 'Task',
      task,
    },
  });

  const { data: members } = useSWR(task.board_id ? `/api/boards/${task.board_id}/members` : null, fetcher);
  const assignee = members?.find((m: any) => m.id === task.assignee_id);

  const style = {
    transition,
    transform: CSS.Transform.toString(transform),
  };

  const summary = task.description?.replace(/<[^>]*>?/gm, '').trim();

  if (isDragging || dragOnly) {
    return (
      <div
        ref={setNodeRef}
        style={style}
        className="min-h-[120px] h-fit rounded-xl border-2 border-rose-500 bg-white/80 p-4 opacity-70 shadow-lg dark:bg-zinc-800/80"
      >
        <h3 className="line-clamp-3 text-sm font-semibold leading-tight">{task.title}</h3>
        {summary && <p className="mt-1.5 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{summary}</p>}
      </div>
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      className={clsx(
        'group relative flex min-h-[132px] h-fit cursor-pointer flex-col justify-start rounded-xl border border-zinc-200 bg-white p-4 pb-14 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md hover:ring-2 hover:ring-inset hover:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-800 dark:shadow-md'
      )}
    >
      <button type="button" onClick={onClick} className="flex w-full flex-1 flex-col justify-start text-left">
        <div className="mb-2 flex flex-wrap items-center gap-2 pr-8">
          <span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${statusTone(task)}`}>
            {task.status || task.list_id}
          </span>
          {task.blocked_reason && (
            <span className="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-[10px] font-semibold text-red-700 dark:bg-red-900/20 dark:text-red-300">
              <AlertTriangle size={10} /> blocked
            </span>
          )}
          <span className="inline-flex items-center gap-1 rounded-full bg-zinc-100 px-2 py-0.5 text-[10px] font-medium text-zinc-600 dark:bg-zinc-700/70 dark:text-zinc-300">
            <Hash size={10} /> {task.id}
          </span>
        </div>

        <h3 className="line-clamp-3 text-sm font-semibold leading-tight">{task.title}</h3>
        {summary && (
          <p className="mt-1.5 line-clamp-2 pr-2 text-xs text-gray-500 dark:text-gray-400">
            {summary}
          </p>
        )}

      </button>

      <div className="absolute inset-x-4 bottom-3 flex items-center justify-between gap-2">
        <div className="min-w-0 pr-2 text-[11px] text-gray-500 dark:text-gray-400">
          <span className="inline-flex max-w-full items-center gap-1 rounded-full bg-zinc-100 px-2 py-1 dark:bg-zinc-700/60">
            <User size={12} className="shrink-0" />
            <span className="truncate">{assignee ? assignee.name : 'Unassigned'}</span>
          </span>
        </div>

        {assignee ? (
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs shadow-sm ring-1 ring-white dark:bg-zinc-700 dark:ring-zinc-800" title={assignee.name}>
            {assignee.avatar}
          </div>
        ) : <div className="h-7 w-7 shrink-0" />}
      </div>

      <button
        type="button"
        {...listeners}
        className="absolute right-2 top-2 flex h-7 w-7 cursor-grab items-center justify-center rounded-lg border bg-white/90 text-gray-500 opacity-80 shadow-sm transition hover:bg-gray-50 hover:text-rose-500 active:cursor-grabbing dark:border-zinc-700 dark:bg-zinc-900/90 dark:text-gray-400 dark:hover:bg-zinc-800"
        aria-label="Drag task"
      >
        <GripVertical size={14} />
      </button>
    </div>
  );
}
