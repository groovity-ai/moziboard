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
}

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export function TaskCard({ task, onClick }: TaskCardProps) {
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

  if (isDragging) {
    return (
      <div
        ref={setNodeRef}
        style={style}
        className="opacity-30 border-2 border-rose-500 min-h-[100px] h-fit items-center flex text-left rounded-xl hover:ring-2 hover:ring-inset hover:ring-rose-500 cursor-grab relative"
      />
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      onClick={onClick}
      className={clsx(
        'group relative flex min-h-[100px] h-fit cursor-pointer flex-col justify-start rounded-xl bg-white p-4 pb-8 shadow-sm hover:ring-2 hover:ring-inset hover:ring-rose-500 dark:bg-zinc-800 dark:shadow-md'
      )}
    >
      <div className="flex w-full flex-col justify-start pr-4">
        <h3 className="line-clamp-3 text-sm font-semibold leading-tight">{task.title}</h3>
        {task.description && (
          <p className="mt-1.5 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
            {task.description.replace(/<[^>]*>?/gm, '') /* Strip basic HTML from rich text */}
          </p>
        )}
      </div>
      
      {/* Assignee Avatar */}
      {assignee && (
        <div className="absolute bottom-2 right-2 flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-xs shadow-sm ring-1 ring-white dark:bg-zinc-700 dark:ring-zinc-800" title={assignee.name}>
          {assignee.avatar}
        </div>
      )}

      <div className="absolute right-2 top-3 opacity-0 group-hover:opacity-100">
        <GripVertical size={14} className="text-gray-400" />
      </div>
    </div>
  );
}
