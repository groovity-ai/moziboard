import React, { useState } from 'react';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { useDroppable } from '@dnd-kit/core';
import { TaskCard } from './TaskCard';
import { ListType, Task } from './Board';
import { mutate } from 'swr';
import { Plus, X } from 'lucide-react';
import clsx from 'clsx';

interface ListContainerProps {
  list: ListType;
  boardId: string;
  onTaskClick?: (task: Task) => void;
  isActiveDropTarget?: boolean;
}

export function ListContainer({ list, boardId, onTaskClick, isActiveDropTarget = false }: ListContainerProps) {
  const [isAdding, setIsAdding] = useState(false);
  const [newTitle, setNewTitle] = useState('');

  const { setNodeRef, isOver } = useDroppable({
    id: list.id,
  });

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
        'flex h-fit w-[350px] shrink-0 flex-col gap-4 rounded-xl border bg-gray-100 p-4 shadow-sm transition-all dark:bg-zinc-900',
        (isOver || isActiveDropTarget)
          ? 'border-rose-400 ring-2 ring-rose-200 dark:border-rose-500 dark:ring-rose-900/30'
          : 'border-transparent'
      )}
    >
      <div className="flex items-center justify-between text-lg font-bold">
        <span>{list.title}</span>
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
          <div className="flex min-h-[100px] items-center justify-center rounded-xl border border-dashed border-gray-300 bg-white/60 text-sm text-gray-400 dark:border-zinc-700 dark:bg-zinc-800/40 dark:text-zinc-500">
            Drop task here
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
            <button
              onClick={handleAddTask}
              className="rounded-md bg-rose-500 px-3 py-1 text-sm text-white hover:bg-rose-600"
            >
              Add
            </button>
            <button
              onClick={() => { setIsAdding(false); setNewTitle(''); }}
              className="rounded-md px-3 py-1 text-sm hover:bg-gray-200 dark:hover:bg-zinc-800"
            >
              <X size={16} />
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => setIsAdding(true)}
          className="flex w-full items-center gap-2 rounded-md p-2 hover:bg-gray-200 dark:hover:bg-zinc-800"
        >
          <Plus size={16} /> Add Task
        </button>
      )}
    </div>
  );
}
