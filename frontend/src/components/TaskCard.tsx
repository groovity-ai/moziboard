import React from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Task } from './Board';
import clsx from 'clsx';
import { GripVertical } from 'lucide-react';
import useSWR from 'swr';

interface TaskCardProps {
  task: Task;
  onClick?: () => void;
  dragOnly?: boolean;
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

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

  if (isDragging || dragOnly) {
    return (
      <div
        ref={setNodeRef}
        style={style}
        className="min-h-[100px] h-fit rounded-xl border-2 border-rose-500 bg-white/80 p-4 opacity-70 shadow-lg dark:bg-zinc-800/80"
      >
        <h3 className="line-clamp-3 text-sm font-semibold leading-tight">{task.title}</h3>
        {task.description && (
          <p className="mt-1.5 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
            {task.description.replace(/<[^>]*>?/gm, '')}
          </p>
        )}
      </div>
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      className={clsx(
        'group relative flex min-h-[100px] h-fit cursor-pointer flex-col justify-start rounded-xl bg-white p-4 pb-8 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md hover:ring-2 hover:ring-inset hover:ring-rose-500 dark:bg-zinc-800 dark:shadow-md'
      )}
    >
      <button
        type="button"
        onClick={onClick}
        className="flex w-full flex-col justify-start pr-8 text-left"
      >
        <h3 className="line-clamp-3 text-sm font-semibold leading-tight">{task.title}</h3>
        {task.description && (
          <p className="mt-1.5 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
            {task.description.replace(/<[^>]*>?/gm, '')}
          </p>
        )}
      </button>

      {assignee && (
        <div className="absolute bottom-2 right-2 flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-xs shadow-sm ring-1 ring-white dark:bg-zinc-700 dark:ring-zinc-800" title={assignee.name}>
          {assignee.avatar}
        </div>
      )}

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
